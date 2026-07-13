package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"neoray/internal/agent"
	"neoray/internal/logger"
	"neoray/internal/session"
)

// writeJSONError writes a properly escaped JSON error response.
func writeJSONError(w http.ResponseWriter, statusCode int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp, _ := json.Marshal(map[string]string{"error": errMsg})
	_, _ = w.Write(resp)
}

// handleChat 处理聊天消息
func (c *Client) handleChat(payload interface{}) {
	var chatPayload ChatPayload
	payloadBytes, _ := json.Marshal(payload)
	if err := json.Unmarshal(payloadBytes, &chatPayload); err != nil {
		c.sendError("invalid_payload", "Invalid chat payload")
		return
	}

	if chatPayload.Message == "" {
		c.sendError("empty_message", "Message cannot be empty")
		return
	}

	// 设置客户端的频道和用户ID
	if chatPayload.ChannelID != "" {
		c.SetChannelID(chatPayload.ChannelID)
	}
	if chatPayload.UserID != "" {
		c.SetUserID(chatPayload.UserID)
	}
	if c.GetChannelID() == "" {
		c.SetChannelID("default")
	}
	if c.GetUserID() == "" {
		c.SetUserID("default")
	}

	// 获取或创建会话
	var sess *session.Session
	var err error

	if chatPayload.SessionID != "" {
		sess, err = c.Server.sessionMgr.GetSessionWithValidation(chatPayload.SessionID, c.GetChannelID(), c.GetUserID())
		if err != nil {
			sess, err = c.Server.sessionMgr.CreateSession(c.GetChannelID(), c.GetUserID())
		}
	} else {
		sess, err = c.Server.sessionMgr.CreateSession(c.GetChannelID(), c.GetUserID())
	}
	if err != nil {
		c.sendError("session_error", "Failed to create or retrieve session")
		return
	}

	c.SetSessionID(sess.ID)

	// 发送开始响应
	c.sendMessage("chat_start", map[string]interface{}{
		"session_id": sess.ID,
	})

	// 调用 Agent 处理（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := c.Server.agent.Chat(ctx, sess, chatPayload.Message)
	if err != nil {
		logger.Error("Agent chat failed", logger.ErrorField(err))
		c.sendError("agent_error", "An internal error occurred while processing your request")
		return
	}

	// 发送最终响应
	response := map[string]interface{}{
		"session_id": sess.ID,
		"content":    result.Message.Content,
	}
	if result.TokenUsage != nil {
		response["token_usage"] = result.TokenUsage
	}
	if result.ToolCalls > 0 {
		response["tool_calls"] = result.ToolCalls
	}
	if result.Iterations > 0 {
		response["iterations"] = result.Iterations
	}

	c.sendMessage("chat_end", response)
}

// handleChatStream 处理流式聊天消息
func (c *Client) handleChatStream(payload interface{}) {
	var chatPayload ChatPayload
	payloadBytes, _ := json.Marshal(payload)
	if err := json.Unmarshal(payloadBytes, &chatPayload); err != nil {
		c.sendError("invalid_payload", "Invalid chat payload")
		return
	}

	if chatPayload.Message == "" {
		c.sendError("empty_message", "Message cannot be empty")
		return
	}

	// 设置客户端的频道和用户ID
	if chatPayload.ChannelID != "" {
		c.SetChannelID(chatPayload.ChannelID)
	}
	if chatPayload.UserID != "" {
		c.SetUserID(chatPayload.UserID)
	}
	if c.GetChannelID() == "" {
		c.SetChannelID("default")
	}
	if c.GetUserID() == "" {
		c.SetUserID("default")
	}

	// 获取或创建会话
	var sess *session.Session
	var err error

	if chatPayload.SessionID != "" {
		sess, err = c.Server.sessionMgr.GetSessionWithValidation(chatPayload.SessionID, c.GetChannelID(), c.GetUserID())
		if err != nil {
			sess, err = c.Server.sessionMgr.CreateSession(c.GetChannelID(), c.GetUserID())
		}
	} else {
		sess, err = c.Server.sessionMgr.CreateSession(c.GetChannelID(), c.GetUserID())
	}
	if err != nil {
		c.sendError("session_error", "Failed to create or retrieve session")
		return
	}

	c.SetSessionID(sess.ID)

	// 发送开始响应
	c.sendMessage("chat_start", map[string]interface{}{
		"session_id": sess.ID,
	})

	// 调用 Agent 流式处理（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	streamChan, err := c.Server.agent.ChatStream(ctx, sess, chatPayload.Message)
	if err != nil {
		logger.Error("Agent chat stream failed", logger.ErrorField(err))
		c.sendError("agent_error", "An internal error occurred while processing your request")
		return
	}

	// 接收流式内容并推送给客户端
	var fullContent string
	for chunk := range streamChan {
		switch chunk.Type {
		case "text":
			fullContent += chunk.Content
			c.sendMessage("chat_chunk", map[string]interface{}{
				"session_id": sess.ID,
				"content":    chunk.Content,
			})
		case "tool_start":
			// 通知客户端工具调用开始
			c.sendMessage("tool_call_start", map[string]interface{}{
				"session_id": sess.ID,
				"tool_calls": chunk.ToolCalls,
			})
		case "tool_result":
			// 通知客户端工具调用结果
			c.sendMessage("tool_call_result", map[string]interface{}{
				"session_id":  sess.ID,
				"tool_result": chunk.ToolResults,
			})
		case "error":
			logger.Error("Stream chunk error", logger.ErrorField(chunk.Error))
			c.sendError("stream_error", chunk.Error.Error())
			return
		case "end":
			// 完成
			response := map[string]interface{}{
				"session_id": sess.ID,
				"content":    fullContent,
			}
			if chunk.SessionMsg != nil && len(chunk.SessionMsg.ToolCalls) > 0 {
				response["tool_calls"] = chunk.SessionMsg.ToolCalls
			}
			c.sendMessage("chat_end", response)
		}
	}
}

// handleCreateSession 处理创建会话
func (c *Client) handleCreateSession(payload interface{}) {
	var createPayload CreateSessionPayload
	payloadBytes, _ := json.Marshal(payload)
	if err := json.Unmarshal(payloadBytes, &createPayload); err != nil {
		c.sendError("invalid_payload", "Invalid create session payload")
		return
	}

	// 设置客户端的频道和用户ID
	if createPayload.ChannelID != "" {
		c.SetChannelID(createPayload.ChannelID)
	}
	if createPayload.UserID != "" {
		c.SetUserID(createPayload.UserID)
	}
	if c.GetChannelID() == "" {
		c.SetChannelID("default")
	}
	if c.GetUserID() == "" {
		c.SetUserID("default")
	}

	title := createPayload.Name
	if title == "" {
		title = "New Session"
	}

	sess, err := c.Server.sessionMgr.CreateSession(c.GetChannelID(), c.GetUserID())
	if err != nil {
		c.sendError("session_error", "Failed to create session")
		return
	}
	if title != "New Session" {
		sess.Title = title
		_ = c.Server.sessionMgr.SaveSession(sess)
	}
	c.SetSessionID(sess.ID)

	c.sendMessage("session_created", map[string]interface{}{
		"session_id": sess.ID,
		"channel_id": sess.ChannelID,
		"user_id":    sess.UserID,
		"name":       sess.Title,
		"created_at": sess.CreatedAt,
	})
}

// handleJoinSession 处理加入会话
func (c *Client) handleJoinSession(payload interface{}) {
	var joinPayload JoinSessionPayload
	payloadBytes, _ := json.Marshal(payload)
	if err := json.Unmarshal(payloadBytes, &joinPayload); err != nil {
		c.sendError("invalid_payload", "Invalid join session payload")
		return
	}

	// 设置客户端的频道和用户ID
	if joinPayload.ChannelID != "" {
		c.SetChannelID(joinPayload.ChannelID)
	}
	if joinPayload.UserID != "" {
		c.SetUserID(joinPayload.UserID)
	}
	if c.GetChannelID() == "" {
		c.SetChannelID("default")
	}
	if c.GetUserID() == "" {
		c.SetUserID("default")
	}

	sess, err := c.Server.sessionMgr.GetSessionWithValidation(joinPayload.SessionID, c.GetChannelID(), c.GetUserID())
	if err != nil {
		c.sendError("session_not_found", "Session not found or access denied")
		return
	}

	c.SetSessionID(sess.ID)

	c.sendMessage("session_joined", map[string]interface{}{
		"session_id": sess.ID,
		"channel_id": sess.ChannelID,
		"user_id":    sess.UserID,
		"name":       sess.Title,
		"messages":   sess.Messages,
	})
}

// handleListSessions 处理列出会话
func (c *Client) handleListSessions(payload interface{}) {
	// 尝试解析负载获取 channel_id 和 user_id
	var listPayload ListSessionsPayload
	payloadBytes, _ := json.Marshal(payload)
	_ = json.Unmarshal(payloadBytes, &listPayload)

	channelID := listPayload.ChannelID
	userID := listPayload.UserID
	if channelID == "" {
		channelID = c.GetChannelID()
	}
	if userID == "" {
		userID = c.GetUserID()
	}
	if channelID == "" {
		channelID = "default"
	}
	if userID == "" {
		userID = "default"
	}

	sessions, err := c.Server.sessionMgr.ListSessionsByChannelAndUser(channelID, userID)
	if err != nil {
		c.sendError("list_error", "Failed to list sessions")
		return
	}

	// 转换为简化格式
	sessionList := make([]map[string]interface{}, 0, len(sessions))
	for _, sess := range sessions {
		sessionList = append(sessionList, map[string]interface{}{
			"id":            sess.ID,
			"channel_id":    sess.ChannelID,
			"user_id":       sess.UserID,
			"name":          sess.Title,
			"created_at":    sess.CreatedAt,
			"updated_at":    sess.UpdatedAt,
			"message_count": len(sess.Messages),
		})
	}

	c.sendMessage("session_list", map[string]interface{}{
		"sessions": sessionList,
	})
}

// REST API Handlers

// handleSessions 会话列表/创建
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 从查询参数或请求头获取 channel_id 和 user_id
	channelID := r.URL.Query().Get("channel_id")
	userID := r.URL.Query().Get("user_id")
	if channelID == "" {
		channelID = r.Header.Get("X-Channel-ID")
	}
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	if channelID == "" {
		channelID = "default"
	}
	if userID == "" {
		userID = "default"
	}

	switch r.Method {
	case http.MethodGet:
		// 列出会话
		sessions, err := s.sessionMgr.ListSessionsByChannelAndUser(channelID, userID)
		if err != nil {
			http.Error(w, `{"error":"Failed to list sessions"}`, http.StatusInternalServerError)
			return
		}

		sessionList := make([]map[string]interface{}, 0, len(sessions))
		for _, sess := range sessions {
			sessionList = append(sessionList, map[string]interface{}{
				"id":            sess.ID,
				"channel_id":    sess.ChannelID,
				"user_id":       sess.UserID,
				"name":          sess.Title,
				"created_at":    sess.CreatedAt,
				"updated_at":    sess.UpdatedAt,
				"message_count": len(sess.Messages),
			})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": sessionList,
		})

	case http.MethodPost:
		// 创建会话 — 限制请求体大小
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
		var req struct {
			ChannelID string `json:"channel_id"`
			UserID    string `json:"user_id"`
			Name      string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		// 使用请求中的或默认的 channel_id 和 user_id
		reqChannelID := req.ChannelID
		reqUserID := req.UserID
		if reqChannelID == "" {
			reqChannelID = channelID
		}
		if reqUserID == "" {
			reqUserID = userID
		}

		title := req.Name
		if title == "" {
			title = "New Session"
		}

		sess, err := s.sessionMgr.CreateSession(reqChannelID, reqUserID)
		if err != nil {
			http.Error(w, `{"error":"Failed to create session"}`, http.StatusInternalServerError)
			return
		}
		if title != "New Session" {
			sess.Title = title
			_ = s.sessionMgr.SaveSession(sess)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         sess.ID,
			"channel_id": sess.ChannelID,
			"user_id":    sess.UserID,
			"name":       sess.Title,
			"created_at": sess.CreatedAt,
		})

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleSession 单个会话操作
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 从查询参数或请求头获取 channel_id 和 user_id
	channelID := r.URL.Query().Get("channel_id")
	userID := r.URL.Query().Get("user_id")
	if channelID == "" {
		channelID = r.Header.Get("X-Channel-ID")
	}
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	if channelID == "" {
		channelID = "default"
	}
	if userID == "" {
		userID = "default"
	}

	// 从 URL 中提取 session ID
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, `{"error":"Invalid session ID"}`, http.StatusBadRequest)
		return
	}
	sessionID := pathParts[3]

	switch r.Method {
	case http.MethodGet:
		// 获取会话详情
		sess, err := s.sessionMgr.GetSessionWithValidation(sessionID, channelID, userID)
		if err != nil {
			http.Error(w, `{"error":"Session not found or access denied"}`, http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(sess)

	case http.MethodPost:
		// 发送聊天消息 — 限制请求体大小防止 DoS
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
		var req struct {
			ChannelID string `json:"channel_id"`
			UserID    string `json:"user_id"`
			Message   string `json:"message"`
			Stream    bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, `{"error":"Message cannot be empty"}`, http.StatusBadRequest)
			return
		}

		// 使用请求中的或默认的 channel_id 和 user_id
		reqChannelID := req.ChannelID
		reqUserID := req.UserID
		if reqChannelID == "" {
			reqChannelID = channelID
		}
		if reqUserID == "" {
			reqUserID = userID
		}

		sess, err := s.sessionMgr.GetSessionWithValidation(sessionID, reqChannelID, reqUserID)
		if err != nil {
			http.Error(w, `{"error":"Session not found or access denied"}`, http.StatusNotFound)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if req.Stream {
			// 流式响应 - 使用 SSE 格式
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			streamChan, err := s.agent.ChatStream(ctx, sess, req.Message)
			if err != nil {
				logger.Error("REST chat stream failed", logger.ErrorField(err))
				writeJSONError(w, http.StatusInternalServerError, "An internal error occurred")
				return
			}

			flusher, ok := w.(http.Flusher)
			if ok {
				for chunk := range streamChan {
					switch chunk.Type {
					case "text":
						eventData, _ := json.Marshal(map[string]interface{}{
							"type":    "text",
							"content": chunk.Content,
						})
						_, _ = w.Write([]byte("data: " + string(eventData) + "\n\n"))
						flusher.Flush()
					case "tool_start":
						eventData, _ := json.Marshal(map[string]interface{}{
							"type":       "tool_start",
							"tool_calls": chunk.ToolCalls,
						})
						_, _ = w.Write([]byte("data: " + string(eventData) + "\n\n"))
						flusher.Flush()
					case "tool_result":
						eventData, _ := json.Marshal(map[string]interface{}{
							"type":        "tool_result",
							"tool_result": chunk.ToolResults,
						})
						_, _ = w.Write([]byte("data: " + string(eventData) + "\n\n"))
						flusher.Flush()
					case "end":
						eventData, _ := json.Marshal(map[string]interface{}{
							"type":    "end",
							"content": chunk.Content,
						})
						_, _ = w.Write([]byte("data: " + string(eventData) + "\n\n"))
						flusher.Flush()
					case "error":
						eventData, _ := json.Marshal(map[string]interface{}{
							"type":  "error",
							"error": chunk.Error.Error(),
						})
						_, _ = w.Write([]byte("data: " + string(eventData) + "\n\n"))
						flusher.Flush()
					}
				}
			}
		} else {
			// 非流式响应
			result, err := s.agent.Chat(ctx, sess, req.Message)
			if err != nil {
				logger.Error("REST chat failed", logger.ErrorField(err))
				writeJSONError(w, http.StatusInternalServerError, "An internal error occurred")
				return
			}

			response := map[string]interface{}{
				"content": result.Message.Content,
			}
			if result.TokenUsage != nil {
				response["token_usage"] = result.TokenUsage
			}
			if result.ToolCalls > 0 {
				response["tool_calls"] = result.ToolCalls
			}

			json.NewEncoder(w).Encode(response)
		}

	case http.MethodDelete:
		// 删除会话
		if err := s.sessionMgr.DeleteSession(sessionID); err != nil {
			http.Error(w, `{"error":"Failed to delete session"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// Server 类型需要更新字段类型
// 更新 server.go 中的 agent 字段为 *agent.Agent
func init() {
	// 确保我们正确导入了新的 agent 包
	var _ agent.Agent
}
