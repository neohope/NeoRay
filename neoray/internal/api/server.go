package api

import (
	"context"
	crypto_rand "crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"neoray/internal/agent"
	"neoray/internal/bus"
	"neoray/internal/channel"
	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/session"
)

// Server API 服务器
type Server struct {
	cfg         *config.Config
	cfgMu       sync.RWMutex // 保护 cfg 的并发读写（P0-3 修复）
	agent       agent.AgentInterface
	sessionMgr  *session.Manager
	channelMgr  *channel.Manager
	msgBus      *bus.MessageBus
	upgrader    websocket.Upgrader
	clients     map[string]*Client
	clientsMu   sync.RWMutex
	httpServer  *http.Server
	rateLimiter *rateLimiter

	// webSessions 为 Web UI 的 Cookie 会话存储（浏览器 WS 无法自定义请求头，
	// 故 Web UI 走 Cookie 鉴权）。为 nil 时表示未启用。
	webSessions *webSessionStore
}

// rateLimiter implements a simple per-IP token bucket rate limiter.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int           // requests per window
	burst    int           // max burst size (bucket capacity)
	window   time.Duration // time window
	stopCh   chan struct{} // signals the cleanup goroutine to exit
}

type visitor struct {
	// tokens 用 float 保存：整数除法会把亚令牌零头丢掉，
	// 而 lastSeen 已经推进 → 间隔不够一个令牌周期的请求永远补不上令牌，
	// 一个每 900ms 请求一次的客户端会撞上"永久 429"。
	tokens   float64
	lastSeen time.Time
}

// newRateLimiterForConfig 依据 security.rate_limit 构造限流器；
// enabled=false 时返回 nil（调用方需判空）
func newRateLimiterForConfig(cfg *config.Config) *rateLimiter {
	if cfg == nil {
		return nil
	}
	r := cfg.Security.RateLimit
	if !r.Enabled {
		return nil
	}
	if r.RequestsPerMinute <= 0 {
		return nil
	}
	return newRateLimiter(r.RequestsPerMinute, r.Burst, time.Minute)
}

func newRateLimiter(rate int, burst int, window time.Duration) *rateLimiter {
	if burst <= 0 {
		burst = rate
	}
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
		window:   window,
		stopCh:   make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(window * 2)
		defer ticker.Stop()
		for {
			select {
			case <-rl.stopCh:
				return
			case <-ticker.C:
				rl.mu.Lock()
				for ip, v := range rl.visitors {
					if time.Since(v.lastSeen) > window*2 {
						delete(rl.visitors, ip)
					}
				}
				rl.mu.Unlock()
			}
		}
	}()
	return rl
}

// stop terminates the background cleanup goroutine.
func (rl *rateLimiter) stop() {
	close(rl.stopCh)
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{tokens: float64(rl.burst - 1), lastSeen: time.Now()}
		return true
	}

	// Refill tokens based on elapsed time (fractional to avoid losing remainders).
	elapsed := time.Since(v.lastSeen)
	v.lastSeen = time.Now()
	perMs := float64(rl.rate) / float64(rl.window.Milliseconds())
	v.tokens += float64(elapsed.Milliseconds()) * perMs
	if v.tokens > float64(rl.burst) {
		v.tokens = float64(rl.burst)
	}

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

// Client WebSocket 客户端
type Client struct {
	// writeTimeout / pingInterval 由 Server 在握手时注入，便于按配置调整
	writeTimeout time.Duration
	pingInterval time.Duration
	ID           string
	Conn         *websocket.Conn
	Send         chan []byte
	Server       *Server

	mu        sync.RWMutex
	ChannelID string
	UserID    string
	SessionID string

	// closed 在连接注销时关闭。用它代替 close(Send)：
	// 处理消息的 goroutine 可能在超时后仍在跑，close(Send) 会让它的
	// sendMessage 触发 "send on closed channel" panic（整个进程一起挂）。
	closed    chan struct{}
	closeOnce sync.Once
}

// markClosed 幂等地标记连接已关闭
func (c *Client) markClosed() {
	if c.closed == nil {
		return
	}
	c.closeOnce.Do(func() { close(c.closed) })
}

// SetChannelID safely sets the client's channel ID.
func (c *Client) SetChannelID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ChannelID = id
}

// GetChannelID safely gets the client's channel ID.
func (c *Client) GetChannelID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ChannelID
}

// SetUserID safely sets the client's user ID.
func (c *Client) SetUserID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.UserID = id
}

// GetUserID safely gets the client's user ID.
func (c *Client) GetUserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.UserID
}

// SetSessionID safely sets the client's session ID.
func (c *Client) SetSessionID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SessionID = id
}

// GetSessionID safely gets the client's session ID.
func (c *Client) GetSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SessionID
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, aiAgent agent.AgentInterface, sessionMgr *session.Manager, channelMgr *channel.Manager, msgBus *bus.MessageBus) *Server {
	s := &Server{
		cfg:        cfg,
		agent:      aiAgent,
		sessionMgr: sessionMgr,
		channelMgr: channelMgr,
		msgBus:     msgBus,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					// P1-2: 仅在开发模式下允许空 Origin（非浏览器客户端）
					// 生产环境拒绝空 Origin，防止绕过 CORS 保护
					if cfg.App.Env == "development" || cfg.App.Env == "dev" {
						return true
					}
					return false
				}
				allowedOrigins := cfg.Server.CORS.AllowedOrigins
				if len(allowedOrigins) == 0 {
					return isLocalhostOrigin(origin)
				}
				for _, allowed := range allowedOrigins {
					if origin == allowed {
						return true
					}
				}
				return false
			},
		},
		clients:     make(map[string]*Client),
		rateLimiter: newRateLimiterForConfig(cfg),
	}

	// 仅在启用 Web UI 时初始化会话存储，避免无用开销
	if cfg.Web.Enabled {
		s.webSessions = newWebSessionStore()
	}

	// 如果有消息总线，订阅出站消息
	if msgBus != nil {
		s.subscribeToBus()
	}

	return s
}

// Start 启动服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// WebSocket 端点 — P1-1: 也通过 wrapMiddleware 添加速率限制
	mux.HandleFunc("/ws", s.wrapMiddleware(s.handleWebSocket))

	// REST API 端点
	mux.HandleFunc("/api/sessions", s.wrapMiddleware(s.handleSessions))
	mux.HandleFunc("/api/sessions/", s.wrapMiddleware(s.handleSession))
	mux.HandleFunc("/api/config", s.wrapMiddleware(s.handleConfig))
	mux.HandleFunc("/api/health", s.wrapMiddleware(s.handleHealth))

	// Web UI 认证端点（仅 Web UI 启用时注册）
	if s.cfg.Web.Enabled {
		// 认证端点只套 CORS + 限流，不套 bearer 认证：
		// 登录接口本身就是换取凭证的入口，不能被 API Key 拦住；
		// 但它必须有和其余 API 相同的限流，否则可被暴力猜密码/灌日志。
		mux.HandleFunc("/api/auth/login", s.wrapRateLimitOnly(s.handleAuthLogin))
		mux.HandleFunc("/api/auth/logout", s.wrapRateLimitOnly(s.handleAuthLogout))
		mux.HandleFunc("/api/auth/me", s.wrapRateLimitOnly(s.handleAuthMe))
	}

	// API 未注册路径兜底：返回 JSON 404。
	// 必须显式注册，否则会被下面的 "/" 静态路由（SPA 回退）吞掉并返回 HTML，
	// 导致前端 JSON 解析失败。Go 1.22+ ServeMux 按最长前缀匹配，
	// 本模式优先级低于上面已注册的精确路径，故不影响现有接口。
	mux.HandleFunc("/api/", s.wrapMiddleware(s.handleAPINotFound))

	// Web UI 静态资源（可选，默认关闭）。
	// 仅在 web.enabled = true 时注册；关闭时行为与改动前完全一致。
	if s.cfg.Web.Enabled {
		s.warnIfWebExposed()
		if handler := newStaticHandler(s.cfg.Web.StaticDir); handler != nil {
			// 注意：静态资源不经过 wrapMiddleware，因此不做认证/限流，
			// 以免页面本体、JS、CSS 被鉴权拦截而无法加载。
			mux.Handle("/", handler)
			logger.Info("Web UI enabled",
				logger.String("static_dir", s.cfg.Web.StaticDir))
		} else {
			logger.Warn("Web UI enabled but no usable assets found; UI will be unavailable")
		}
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	// 超时取自配置（server.read_timeout / write_timeout / idle_timeout），
	// 未配置时保留历史默认值
	readTimeout := 15 * time.Second
	writeTimeout := 60 * time.Second
	idleTimeout := 60 * time.Second
	if s.cfg != nil {
		if s.cfg.Server.ReadTimeout > 0 {
			readTimeout = s.cfg.Server.ReadTimeout
		}
		if s.cfg.Server.WriteTimeout > 0 {
			writeTimeout = s.cfg.Server.WriteTimeout
		}
		if s.cfg.Server.IdleTimeout > 0 {
			idleTimeout = s.cfg.Server.IdleTimeout
		}
	}

	s.httpServer = &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: readTimeout,
		// P1-fix: 使用合理的 WriteTimeout 替代 0。SSE handler 内部已有 context 超时控制，
		// 但 WriteTimeout=0 会导致所有 HTTP 连接（不仅是 SSE）无写超时保护。
		// SSE handler 通过 http.NewResponseController 动态延长写超时。
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// 检查认证配置
	if s.cfg.Security.Auth.Enabled && s.cfg.Security.Auth.SecretKey == "" {
		logger.Warn("Auth is enabled but SecretKey is empty — all requests will be rejected. Set security.auth.secret_key in neoray.toml.")
	}

	logger.Info("API server starting",
		logger.String("addr", addr),
		logger.Duration("ws_message_timeout", s.effectiveWSMessageTimeout()),
	)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", logger.ErrorField(err))
		}
	}()

	return nil
}

// effectiveWSMessageTimeout 返回单条 WebSocket 消息的生效处理超时。
// 优先取配置值；未配置或非法时回退到默认 10 分钟。
// 抽取为方法是为了便于启动日志与 readPump 共用同一来源，避免两处不一致。
// effectiveChatTimeout 单次对话处理的超时。
//
// 之前 Web UI 的三条路径各自写死 5 分钟：比 agent 自身 10 分钟的上限更短，
// 长任务在 Web UI 会被提前取消，而飞书侧却能跑完。现在统一读
// agent.message_timeout（0 时沿用 agent 默认 10 分钟）。
func (s *Server) effectiveChatTimeout() time.Duration {
	const fallback = 10 * time.Minute
	if cfg := s.getConfig(); cfg != nil && cfg.Agent.MessageTimeout > 0 {
		return cfg.Agent.MessageTimeout
	}
	return fallback
}

// effectiveWSWriteTimeout 单条 WS 消息写超时（默认 10s）
func (s *Server) effectiveWSWriteTimeout() time.Duration {
	const defaultTimeout = 10 * time.Second
	if cfg := s.getConfig(); cfg != nil && cfg.Server.WSWriteTimeout > 0 {
		return cfg.Server.WSWriteTimeout
	}
	return defaultTimeout
}

// effectiveWSPingInterval 服务端 WS 心跳间隔（默认 30s）
func (s *Server) effectiveWSPingInterval() time.Duration {
	const defaultInterval = 30 * time.Second
	if cfg := s.getConfig(); cfg != nil && cfg.Server.WSPingInterval > 0 {
		return cfg.Server.WSPingInterval
	}
	return defaultInterval
}

func (s *Server) effectiveWSMessageTimeout() time.Duration {
	const defaultTimeout = 10 * time.Minute
	if cfg := s.getConfig(); cfg != nil && cfg.Server.WSMessageTimeout > 0 {
		return cfg.Server.WSMessageTimeout
	}
	return defaultTimeout
}

// subscribeToBus 订阅消息总线
func (s *Server) subscribeToBus() {
	// 创建订阅通道
	outChan := make(chan *bus.OutboundMessage, bus.OutboundSubscriberBuffer)

	// 订阅总线
	if err := s.msgBus.SubscribeOutbound("api_server", outChan); err != nil {
		logger.Warn("Failed to subscribe API server to bus", logger.ErrorField(err))
		return
	}

	// 启动协程处理出站消息
	go func() {
		for msg := range outChan {
			// 广播给所有 WebSocket 客户端，或者目标是 websocket 频道
			if msg.ChannelID == "" || msg.ChannelID == "websocket" {
				// 根据总线消息类型决定 WebSocket 消息类型
				wsMsgType := string(msg.Type)
				payload := make(map[string]interface{})

				// 构建 payload
				payload["session_id"] = msg.SessionID
				if msg.Content != "" {
					payload["content"] = msg.Content
				}
				if msg.Metadata != nil {
					for k, v := range msg.Metadata {
						payload[k] = v
					}
				}

				// 向后兼容处理：如果是旧类型，映射到新类型
				switch msg.Type {
				case bus.MessageTypeDelta:
					wsMsgType = "chat_chunk"
				case bus.MessageTypeAssistant:
					wsMsgType = "chat_end"
				case bus.MessageTypeSystem:
					wsMsgType = "progress"
				case bus.MessageTypeToolCall:
					wsMsgType = "tool_call_start"
				case bus.MessageTypeToolResult:
					wsMsgType = "tool_call_result"
				}

				// 在锁内收集待发送的客户端列表，释放锁后再发送
				// 避免在持有读锁时阻塞在 channel 满的客户端上
				s.clientsMu.RLock()
				var targetClients []*Client
				for _, client := range s.clients {
					if msg.SessionID == "" || msg.SessionID == client.GetSessionID() {
						targetClients = append(targetClients, client)
					}
				}
				s.clientsMu.RUnlock()

				// 释放锁后逐个发送消息
				for _, client := range targetClients {
					client.sendMessage(wsMsgType, payload)
				}
			}
		}
	}()

	logger.Info("API server subscribed to message bus")
}

// getConfig 返回当前配置指针。使用 copy-on-write 模式：
// handleConfig 替换整个指针而非原地修改字段，因此读取方无需深拷贝。
func (s *Server) getConfig() *config.Config {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("API server stopping")

	// Stop rate limiter cleanup goroutine
	if s.rateLimiter != nil {
		s.rateLimiter.stop()
	}

	// Stop Web UI session store cleanup goroutine
	if s.webSessions != nil {
		s.webSessions.stop()
	}

	// 取消消息总线订阅（关闭 outChan 让 subscribeToBus goroutine 退出）
	if s.msgBus != nil {
		s.msgBus.UnsubscribeOutbound("api_server")
	}

	// 关闭所有客户端连接。
	// 用 markClosed 而不是 close(Send)：handler goroutine 可能仍在写，
	// 关闭发送通道会让它 send on closed channel 直接 panic。
	s.clientsMu.Lock()
	for _, client := range s.clients {
		client.markClosed()
		client.Conn.Close()
	}
	s.clients = make(map[string]*Client)
	s.clientsMu.Unlock()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// wrapMiddleware 包装中间件
// wrapRateLimitOnly CORS + 限流，但不做 bearer 认证（供 /api/auth/* 使用）
func (s *Server) wrapRateLimitOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.corsMiddleware(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		clientIP := extractIP(r.RemoteAddr)
		if s.rateLimiter != nil && !s.rateLimiter.allow(clientIP) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Rate limit exceeded. Try again later."}`))
			return
		}

		next(w, r)
	}
}

func (s *Server) wrapMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CORS 中间件
		s.corsMiddleware(w, r)

		// 如果是预检请求，直接返回
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 速率限制中间件 — 使用 RemoteAddr 防止 X-Forwarded-For 伪造绕过
		// rateLimiter 为 nil 表示 security.rate_limit.enabled=false
		clientIP := extractIP(r.RemoteAddr)
		if s.rateLimiter != nil && !s.rateLimiter.allow(clientIP) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Rate limit exceeded. Try again later."}`))
			return
		}

		// 认证中间件
		if !s.authenticateRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Unauthorized: invalid or missing API key"}`))
			return
		}

		// 日志中间件
		start := time.Now()
		logger.Info("API request",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("remote", r.RemoteAddr),
		)

		// 调用实际处理函数
		next(w, r)

		// 记录完成时间
		logger.Info("API request completed",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.Duration("duration", time.Since(start)),
		)
	}
}

// corsMiddleware CORS 中间件 — 只反射受信 origin，不反射任意请求 origin。
func (s *Server) corsMiddleware(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Same-origin request — no CORS header needed
		return
	}

	cfg := s.getConfig()
	allowedOrigins := cfg.Server.CORS.AllowedOrigins
	allowed := false
	for _, o := range allowedOrigins {
		if origin == o {
			allowed = true
			break
		}
	}
	// Web UI 开发模式：允许前端 dev server 的 Origin（仅本地调试用）。
	if !allowed && cfg.Web.DevOrigin != "" && origin == cfg.Web.DevOrigin {
		allowed = true
	}
	if !allowed {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Api-Key")
	if cfg.Server.CORS.AllowCredentials || (cfg.Web.DevOrigin != "" && origin == cfg.Web.DevOrigin) {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// authenticateRequest checks the API key if auth is enabled.
// Returns true if authentication succeeds or is disabled.
func (s *Server) authenticateRequest(r *http.Request) bool {
	cfg := s.getConfig()
	if !cfg.Security.Auth.Enabled {
		return true
	}
	expectedKey := cfg.Security.Auth.SecretKey
	if expectedKey == "" {
		// Auth 启用但密钥为空 — 拒绝所有请求，防止开发模式下无认证保护
		logger.Warn("Request rejected: auth enabled but SecretKey is empty",
			logger.String("remote", r.RemoteAddr),
			logger.String("path", r.URL.Path))
		return false
	}

	// Check Authorization: Bearer <key>
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		const prefix = "Bearer "
		if len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix {
			if subtle.ConstantTimeCompare([]byte(authHeader[len(prefix):]), []byte(expectedKey)) == 1 {
				return true
			}
		}
	}

	// Check X-Api-Key header
	if apiKey := r.Header.Get("X-Api-Key"); subtle.ConstantTimeCompare([]byte(apiKey), []byte(expectedKey)) == 1 {
		return true
	}

	// Check Web UI session cookie —— 浏览器 WebSocket 无法自定义请求头，
	// 因此 Web UI 使用同源 Cookie 鉴权（对存量 Header 客户端无影响）。
	if s.validWebSession(r) {
		return true
	}

	return false
}

// handleWebSocket WebSocket 处理
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 认证检查已由 wrapMiddleware 处理

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed", logger.ErrorField(err))
		return
	}

	clientID := generateClientID()
	client := &Client{
		ID:           clientID,
		Conn:         conn,
		Send:         make(chan []byte, 256),
		Server:       s,
		writeTimeout: s.effectiveWSWriteTimeout(),
		pingInterval: s.effectiveWSPingInterval(),
		closed:       make(chan struct{}),
	}

	s.clientsMu.Lock()
	s.clients[clientID] = client
	s.clientsMu.Unlock()

	logger.Info("WebSocket client connected", logger.String("client_id", clientID))

	// 启动读写 goroutine
	go client.readPump()
	go client.writePump()
}

// readPump 读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.Conn.Close()
		c.Server.removeClient(c.ID)
	}()

	c.Conn.SetReadLimit(64 * 1024) // 64 KB max message size
	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Per-connection rate limiter: max 30 messages per minute
	msgTokens := 30.0
	lastRefill := time.Now()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket read error", logger.ErrorField(err))
			}
			break
		}

		// Refill tokens
		now := time.Now()
		elapsed := now.Sub(lastRefill)
		lastRefill = now
		// 用浮点累计：整数截断会把零头丢掉却推进 lastRefill，
		// 间隔略小于"一个令牌的周期"时永远补不上（30/min 下 1.5s 一次会永久限流）。
		msgTokens += elapsed.Seconds() * 0.5 // 30 per minute
		if msgTokens > 30 {
			msgTokens = 30
		}
		if msgTokens <= 0 {
			c.sendError("rate_limit", "Too many messages. Please slow down.")
			continue
		}
		msgTokens--

		logger.Debug("Received WebSocket message",
			logger.String("client_id", c.ID),
			logger.Int("message_bytes", len(message)),
		)

		// 处理消息（带超时控制，防止消息处理阻塞过久）。
		// 超时值可配（server.ws_message_timeout，默认 10m）：
		// 必须大于 agent 内部的处理超时（5 分钟），否则多轮工具调用的长任务
		// 会被外层提前截断，导致 chat_end 无法下发、前端只能报超时。
		msgTimeout := c.Server.effectiveWSMessageTimeout()
		handleCtx, handleCancel := context.WithTimeout(context.Background(), msgTimeout)
		done := make(chan struct{})
		go func() {
			defer close(done)
			// 单条消息处理 panic 不应带走整个进程
			defer func() {
				if r := recover(); r != nil {
					logger.Error("WebSocket message handler panicked",
						logger.String("client_id", c.ID),
						logger.String("panic", fmt.Sprintf("%v", r)))
					c.sendError("internal_error", "Failed to handle message")
				}
			}()
			c.handleMessage(handleCtx, message)
		}()

		select {
		case <-done:
			// 消息处理完成
		case <-handleCtx.Done():
			// handleCtx 已到期：handler 内部继承了这个 ctx，会被真正中断
			logger.Warn("WebSocket message handling timed out",
				logger.String("client_id", c.ID),
				logger.Int("message_bytes", len(message)),
				logger.Duration("timeout", msgTimeout),
			)
			c.sendError("timeout", "Message handling timed out")

			// 等 handler 真正退出再读下一条：否则同一个连接上会并发处理多条消息，
			// 既可能把会话写乱，也会让超时的 goroutine 越堆越多。
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				logger.Warn("Handler still running after timeout, continuing",
					logger.String("client_id", c.ID))
			}
		}
		handleCancel()
	}
}

// writePump 写入消息到客户端
func (c *Client) writePump() {
	writeTimeout := c.writeTimeout
	if writeTimeout <= 0 {
		writeTimeout = 10 * time.Second
	}
	pingInterval := c.pingInterval
	if pingInterval <= 0 {
		pingInterval = 30 * time.Second
	}
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case <-c.closed:
			return
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// 批量发送队列中的消息
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(ctx context.Context, data []byte) {
	var msg WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("invalid_message", "Failed to parse message")
		return
	}

	switch msg.Type {
	case "chat":
		c.handleChat(ctx, msg.Payload)
	case "chat_stream":
		c.handleChatStream(ctx, msg.Payload)
	case "create_session":
		c.handleCreateSession(msg.Payload)
	case "join_session":
		c.handleJoinSession(msg.Payload)
	case "list_sessions":
		c.handleListSessions(msg.Payload)
	default:
		c.sendError("unknown_type", "Unknown message type: "+msg.Type)
	}
}

// sendMessage 发送消息给客户端
func (c *Client) sendMessage(msgType string, payload interface{}) {
	msg := WebSocketMessage{
		Type:    msgType,
		Payload: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal WebSocket message", logger.ErrorField(err))
		return
	}
	select {
	case <-c.closed:
		// 连接已注销，丢弃消息而不是往已关闭的连接写
		return
	default:
	}

	select {
	case c.Send <- data:
	default:
		logger.Warn("Client send queue full, dropping message",
			logger.String("client_id", c.ID),
		)
	}
}

// sendError 发送错误消息
func (c *Client) sendError(code, message string) {
	c.sendMessage("error", ErrorPayload{
		Code:    code,
		Message: message,
	})
}

// removeClient 移除客户端
func (s *Server) removeClient(clientID string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if client, ok := s.clients[clientID]; ok {
		// 只发关闭信号，不 close(Send)：仍可能有 handler goroutine 在写
		client.markClosed()
		delete(s.clients, clientID)
		logger.Info("WebSocket client disconnected", logger.String("client_id", clientID))
	}
}

// WebSocketMessage WebSocket 消息格式
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// ErrorPayload 错误负载
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ChatPayload 聊天负载
type ChatPayload struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// CreateSessionPayload 创建会话负载
type CreateSessionPayload struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
}

// JoinSessionPayload 加入会话负载
type JoinSessionPayload struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

// ListSessionsPayload 列出会话负载
type ListSessionsPayload struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// generateClientID 生成不可预测的客户端 ID
func generateClientID() string {
	b := make([]byte, 16)
	_, _ = crypto_rand.Read(b)
	return hex.EncodeToString(b)
}

// extractIP 从 RemoteAddr 中提取纯 IP 地址（去除端口）
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// 如果解析失败，返回原始地址
		return remoteAddr
	}
	return host
}

// isLocalhostOrigin 检查 origin 是否为本地地址（防止子域名绕过）
// 只允许 http(s)://localhost、http(s)://127.0.0.1，可选端口或路径
func isLocalhostOrigin(origin string) bool {
	allowedHosts := []string{
		"http://localhost",
		"https://localhost",
		"http://127.0.0.1",
		"https://127.0.0.1",
	}
	for _, prefix := range allowedHosts {
		if !strings.HasPrefix(origin, prefix) {
			continue
		}
		rest := origin[len(prefix):]
		// rest 必须为空、以 : 开头（端口）、或以 / 开头（路径）
		if rest == "" || rest[0] == ':' || rest[0] == '/' {
			return true
		}
	}
	return false
}
