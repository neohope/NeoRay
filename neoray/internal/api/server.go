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
	agent       agent.AgentInterface
	sessionMgr  *session.Manager
	channelMgr  *channel.Manager
	msgBus      *bus.MessageBus
	upgrader    websocket.Upgrader
	clients     map[string]*Client
	clientsMu   sync.RWMutex
	httpServer  *http.Server
	rateLimiter *rateLimiter
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
	tokens    int
	lastSeen  time.Time
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
		rl.visitors[ip] = &visitor{tokens: rl.burst - 1, lastSeen: time.Now()}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := time.Since(v.lastSeen)
	v.lastSeen = time.Now()
	v.tokens += int(elapsed.Seconds() * float64(rl.rate) / rl.window.Seconds())
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}

	if v.tokens <= 0 {
		return false
	}
	v.tokens--
	return true
}

// Client WebSocket 客户端
type Client struct {
	ID         string
	Conn       *websocket.Conn
	Send       chan []byte
	Server     *Server

	mu         sync.RWMutex
	ChannelID  string
	UserID     string
	SessionID  string
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
					return true // Same-origin or non-browser client
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
		rateLimiter: newRateLimiter(cfg.Security.RateLimit.RequestsPerMinute, cfg.Security.RateLimit.Burst, time.Minute),
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

	// WebSocket 端点
	mux.HandleFunc("/ws", s.handleWebSocket)

	// REST API 端点
	mux.HandleFunc("/api/sessions", s.wrapMiddleware(s.handleSessions))
	mux.HandleFunc("/api/sessions/", s.wrapMiddleware(s.handleSession))
	mux.HandleFunc("/api/config", s.wrapMiddleware(s.handleConfig))
	mux.HandleFunc("/api/health", s.wrapMiddleware(s.handleHealth))


	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout 设置为 0，由各 handler 的 context 超时控制（SSE 流式最长 5 分钟）。
		// 原来的 15s 会截断所有 SSE 流。
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	// 检查认证配置
	if s.cfg.Security.Auth.Enabled && s.cfg.Security.Auth.SecretKey == "" {
		logger.Warn("Auth is enabled but SecretKey is empty — all requests will be rejected. Set security.auth.secret_key in neoray.toml.")
	}

	logger.Info("API server starting",
		logger.String("addr", addr),
	)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", logger.ErrorField(err))
		}
	}()

	return nil
}

// subscribeToBus 订阅消息总线
func (s *Server) subscribeToBus() {
	// 创建订阅通道
	outChan := make(chan *bus.OutboundMessage, 50)

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

// publishToBus 发布入站消息到总线
func (s *Server) publishToBus(msg *bus.InboundMessage) error {
	if s.msgBus == nil {
		return nil
	}
	return s.msgBus.PublishInbound(msg)
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("API server stopping")

	// Stop rate limiter cleanup goroutine
	if s.rateLimiter != nil {
		s.rateLimiter.stop()
	}

	// 取消消息总线订阅（关闭 outChan 让 subscribeToBus goroutine 退出）
	if s.msgBus != nil {
		s.msgBus.UnsubscribeOutbound("api_server")
	}

	// 关闭所有客户端连接
	s.clientsMu.Lock()
	for _, client := range s.clients {
		close(client.Send)
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
		clientIP := extractIP(r.RemoteAddr)
		if !s.rateLimiter.allow(clientIP) {
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

	allowedOrigins := s.cfg.Server.CORS.AllowedOrigins
	allowed := false
	for _, o := range allowedOrigins {
		if origin == o {
			allowed = true
			break
		}
	}
	if !allowed {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	if s.cfg.Server.CORS.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// authenticateRequest checks the API key if auth is enabled.
// Returns true if authentication succeeds or is disabled.
func (s *Server) authenticateRequest(r *http.Request) bool {
	if !s.cfg.Security.Auth.Enabled {
		return true
	}
	expectedKey := s.cfg.Security.Auth.SecretKey
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

	return false
}

// handleWebSocket WebSocket 处理
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 认证检查
	if !s.authenticateRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Unauthorized: invalid or missing API key"}`))
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed", logger.ErrorField(err))
		return
	}

	clientID := generateClientID()
	client := &Client{
		ID:     clientID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Server: s,
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
	msgTokens := 30
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
		msgTokens += int(elapsed.Seconds() * 30 / 60) // 30 per minute
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

		// 处理消息（带超时控制，防止消息处理阻塞过久）
		handleCtx, handleCancel := context.WithTimeout(context.Background(), 30*time.Second)
		done := make(chan struct{})
		go func() {
			c.handleMessage(message)
			close(done)
		}()

		select {
		case <-done:
			// 消息处理完成
		case <-handleCtx.Done():
			logger.Warn("WebSocket message handling timed out",
				logger.String("client_id", c.ID),
				logger.Int("message_bytes", len(message)),
			)
			c.sendError("timeout", "Message handling timed out")
		}
		handleCancel()
	}
}

// writePump 写入消息到客户端
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(data []byte) {
	var msg WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("invalid_message", "Failed to parse message")
		return
	}

	switch msg.Type {
	case "chat":
		c.handleChat(msg.Payload)
	case "chat_stream":
		c.handleChatStream(msg.Payload)
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
		close(client.Send)
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
