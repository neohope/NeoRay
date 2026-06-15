package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	agent       *agent.Agent
	sessionMgr  *session.Manager
	channelMgr  *channel.Manager
	msgBus      *bus.MessageBus
	upgrader    websocket.Upgrader
	clients     map[string]*Client
	clientsMu   sync.RWMutex
	httpServer  *http.Server
}

// Client WebSocket 客户端
type Client struct {
	ID         string
	Conn       *websocket.Conn
	Send       chan []byte
	ChannelID  string
	UserID     string
	SessionID  string
	Server     *Server
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, aiAgent *agent.Agent, sessionMgr *session.Manager, channelMgr *channel.Manager, msgBus *bus.MessageBus) *Server {
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
				return true // 允许所有来源，生产环境应限制
			},
		},
		clients: make(map[string]*Client),
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
	mux.HandleFunc("/api/health", s.wrapMiddleware(s.handleHealth))


	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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
				s.clientsMu.RLock()
				for _, client := range s.clients {
					// 如果消息有 SessionID，只发给对应会话的客户端
					if msg.SessionID == "" || msg.SessionID == client.SessionID {
						client.sendMessage("chat", map[string]interface{}{
							"content": msg.Content,
							"type":    msg.Type,
						})
					}
				}
				s.clientsMu.RUnlock()
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

		// 日志中间件
		start := time.Now()
		logger.Info("API request",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.String("remote", r.RemoteAddr),
		)

		// 如果是预检请求，直接返回
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

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

// corsMiddleware CORS 中间件
func (s *Server) corsMiddleware(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

// handleWebSocket WebSocket 处理
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
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

	c.Conn.SetReadLimit(4096)
	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket read error", logger.ErrorField(err))
			}
			break
		}

		logger.Debug("Received WebSocket message",
			logger.String("client_id", c.ID),
			logger.String("message", string(message)),
		)

		// 处理消息
		c.handleMessage(message)
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
	data, _ := json.Marshal(msg)
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

// generateClientID 生成客户端 ID
func generateClientID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
