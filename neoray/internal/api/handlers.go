package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"neoray/internal/logger"
	"neoray/internal/session"
)

// validSessionIDRe 会话 ID 格式校验（只允许字母、数字、下划线、连字符）
var validSessionIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// writeJSONError writes a properly escaped JSON error response.
func writeJSONError(w http.ResponseWriter, statusCode int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp, err := json.Marshal(map[string]string{"error": errMsg})
	if err != nil {
		// Fallback: manually constructed JSON is always valid for a single string value
		resp = []byte(`{"error":"` + strings.ReplaceAll(errMsg, `"`, `\"`) + `"}`)
	}
	_, _ = w.Write(resp)
}

// writeJSON encodes v as JSON into the response writer. Logs errors instead of silently ignoring them.
func writeJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Warn("Failed to encode JSON response", logger.ErrorField(err))
	}
}

// parseAndPrepareChat 解析聊天 payload，设置客户端身份，获取或创建会话
func (c *Client) parseAndPrepareChat(payload interface{}) (*ChatPayload, *session.Session, bool) {
	var chatPayload ChatPayload
	payloadBytes, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		c.sendError("invalid_payload", "Invalid chat payload")
		return nil, nil, false
	}
	if err := json.Unmarshal(payloadBytes, &chatPayload); err != nil {
		c.sendError("invalid_payload", "Invalid chat payload")
		return nil, nil, false
	}

	if chatPayload.Message == "" {
		c.sendError("empty_message", "Message cannot be empty")
		return nil, nil, false
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
			c.sendError("session_not_found", "Session not found or access denied")
			return nil, nil, false
		}
	} else {
		sess, err = c.Server.sessionMgr.CreateSession(c.GetChannelID(), c.GetUserID())
	}
	if err != nil {
		c.sendError("session_error", "Failed to create or retrieve session")
		return nil, nil, false
	}

	c.SetSessionID(sess.ID)
	return &chatPayload, sess, true
}

// handleChat 处理聊天消息
func (c *Client) handleChat(payload interface{}) {
	chatPayload, sess, ok := c.parseAndPrepareChat(payload)
	if !ok {
		return
	}

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
	}
	if result.Message != nil {
		response["content"] = result.Message.Content
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
	chatPayload, sess, ok := c.parseAndPrepareChat(payload)
	if !ok {
		return
	}

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
			c.sendError("stream_error", "An error occurred during streaming")
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
	payloadBytes, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		c.sendError("invalid_payload", "Invalid create session payload")
		return
	}
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
		if err := c.Server.sessionMgr.SaveSession(sess); err != nil {
			logger.Warn("Failed to save session title", logger.ErrorField(err))
		}
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
	payloadBytes, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		c.sendError("invalid_payload", "Invalid join session payload")
		return
	}
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
	payloadBytes, marshalErr := json.Marshal(payload)
	if marshalErr == nil {
		_ = json.Unmarshal(payloadBytes, &listPayload)
	}

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
			writeJSONError(w, http.StatusInternalServerError, "Failed to list sessions")
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

		writeJSON(w,map[string]interface{}{
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
			writeJSONError(w, http.StatusBadRequest, "Invalid request body")
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
			writeJSONError(w, http.StatusInternalServerError, "Failed to create session")
			return
		}
		if title != "New Session" {
			sess.Title = title
			_ = s.sessionMgr.SaveSession(sess)
		}

		w.WriteHeader(http.StatusCreated)
		writeJSON(w,map[string]interface{}{
			"id":         sess.ID,
			"channel_id": sess.ChannelID,
			"user_id":    sess.UserID,
			"name":       sess.Title,
			"created_at": sess.CreatedAt,
		})

	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
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
		writeJSONError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}
	sessionID := pathParts[3]

	// 验证 session ID 格式，防止路径遍历等攻击
	if sessionID == "" || !validSessionIDRe.MatchString(sessionID) {
		writeJSONError(w, http.StatusBadRequest, "Invalid session ID format. Only alphanumeric, underscore and hyphen are allowed.")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 获取会话详情
		sess, err := s.sessionMgr.GetSessionWithValidation(sessionID, channelID, userID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "Session not found or access denied")
			return
		}

		writeJSON(w,sess)

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
			writeJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Message == "" {
			writeJSONError(w, http.StatusBadRequest, "Message cannot be empty")
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
			writeJSONError(w, http.StatusNotFound, "Session not found or access denied")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if req.Stream {
			// 流式响应 - 使用 SSE 格式
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			streamChan, err := s.agent.ChatStream(ctx, sess, req.Message)
			if err != nil {
				logger.Error("REST chat stream failed", logger.ErrorField(err))
				writeJSONError(w, http.StatusInternalServerError, "An internal error occurred")
				return
			}

			flusher, ok := w.(http.Flusher)
			if ok {
				for chunk := range streamChan {
					var eventData []byte
					var marshalErr error
					switch chunk.Type {
					case "text":
						eventData, marshalErr = json.Marshal(map[string]interface{}{
							"type":    "text",
							"content": chunk.Content,
						})
					case "tool_start":
						eventData, marshalErr = json.Marshal(map[string]interface{}{
							"type":       "tool_start",
							"tool_calls": chunk.ToolCalls,
						})
					case "tool_result":
						eventData, marshalErr = json.Marshal(map[string]interface{}{
							"type":        "tool_result",
							"tool_result": chunk.ToolResults,
						})
					case "end":
						eventData, marshalErr = json.Marshal(map[string]interface{}{
							"type":    "end",
							"content": chunk.Content,
						})
					case "error":
						logger.Error("SSE stream error", logger.ErrorField(chunk.Error))
						eventData, marshalErr = json.Marshal(map[string]interface{}{
							"type":  "error",
							"error": "An error occurred during streaming",
						})
					default:
						continue
					}
					if marshalErr != nil {
						logger.Error("Failed to marshal SSE chunk", logger.ErrorField(marshalErr))
						continue
					}
					_, _ = w.Write([]byte("data: " + string(eventData) + "\n\n"))
					flusher.Flush()
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

			response := map[string]interface{}{}
			if result.Message != nil {
				response["content"] = result.Message.Content
			}
			if result.TokenUsage != nil {
				response["token_usage"] = result.TokenUsage
			}
			if result.ToolCalls > 0 {
				response["tool_calls"] = result.ToolCalls
			}

			writeJSON(w,response)
		}

	case http.MethodDelete:
		// 删除会话 — 先验证权限
		if _, err := s.sessionMgr.GetSessionWithValidation(sessionID, channelID, userID); err != nil {
			writeJSONError(w, http.StatusNotFound, "Session not found or access denied")
			return
		}
		if err := s.sessionMgr.DeleteSession(sessionID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to delete session")
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w,map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// maskAPIKey 将 API Key 掩码为 ***last4 格式
func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return "***"
	}
	return "***" + key[len(key)-4:]
}

// buildConfigResponse 构建配置响应（掩码敏感字段）
func (s *Server) buildConfigResponse() map[string]interface{} {
	cfg := s.cfg

	// 构建 providers 列表，掩码 API Key
	providers := make(map[string]interface{})
	for name, p := range cfg.LLM.Providers {
		providers[name] = map[string]interface{}{
			"api_key":             maskAPIKey(p.APIKey),
			"api_url":             p.APIURL,
			"model":               p.Model,
			"max_tokens":          p.MaxTokens,
			"temperature":         p.Temperature,
			"timeout":             p.Timeout.Seconds(),
			"reasoning_effort":    p.ReasoningEffort,
			"prompt_cache_enabled": p.PromptCacheEnabled,
		}
	}

	// 构建 fallback models
	fallbackModels := make([]map[string]interface{}, 0, len(cfg.LLM.FallbackModels))
	for _, fm := range cfg.LLM.FallbackModels {
		fallbackModels = append(fallbackModels, map[string]interface{}{
			"model":            fm.Model,
			"provider":         fm.Provider,
			"max_tokens":       fm.MaxTokens,
			"temperature":      fm.Temperature,
			"reasoning_effort": fm.ReasoningEffort,
		})
	}

	// 构建 channels（掩码 secret）
	channels := map[string]interface{}{
		"websocket": map[string]interface{}{
			"enabled": cfg.Channels.WebSocket.Enabled,
		},
		"feishu": map[string]interface{}{
			"enabled":     cfg.Channels.Feishu.Enabled,
			"app_id":      cfg.Channels.Feishu.AppID,
			"app_secret":  maskAPIKey(cfg.Channels.Feishu.AppSecret),
		},
	}

	// 构建 tools
	tools := map[string]interface{}{
		"shell": map[string]interface{}{
			"enabled": cfg.Tools.Shell.Enabled,
		},
		"web": map[string]interface{}{
			"enabled": cfg.Tools.Web.Enabled,
		},
		"cron": map[string]interface{}{
			"enabled": cfg.Tools.Cron.Enabled,
		},
	}

	return map[string]interface{}{
		"llm": map[string]interface{}{
			"default_provider": cfg.LLM.DefaultProvider,
			"providers":        providers,
			"fallback_models":  fallbackModels,
		},
		"channels": channels,
		"tools":    tools,
	}
}

// isAdminRequest 检查请求是否具有管理员权限
// 需要通过 X-Admin-Token 头提供管理员令牌，该令牌与 API Key 不同
func (s *Server) isAdminRequest(r *http.Request) bool {
	adminToken := s.cfg.Security.Auth.AdminToken
	if adminToken == "" {
		// 如果未配置 AdminToken，拒绝所有配置修改请求
		logger.Warn("Config update rejected: admin_token not configured")
		return false
	}

	// 检查 X-Admin-Token 头
	token := r.Header.Get("X-Admin-Token")
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) == 1
}

// handleConfig 处理配置读取和更新
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// 返回当前配置（敏感字段已掩码）
		writeJSON(w,s.buildConfigResponse())

	case http.MethodPut:
		// 更新配置需要管理员权限 — 防止普通 API Key 修改 LLM provider 指向恶意服务器
		if !s.isAdminRequest(r) {
			writeJSONError(w, http.StatusForbidden, "Admin access required for config updates. Use X-Admin-Token header.")
			return
		}

		// 更新配置 — 限制请求体大小
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// 审计日志：记录变更的字段
		var changedFields []string

		// 更新 LLM 配置
		if llm, ok := req["llm"].(map[string]interface{}); ok {
			if dp, ok := llm["default_provider"].(string); ok {
				changedFields = append(changedFields, "llm.default_provider")
				s.cfg.LLM.DefaultProvider = dp
			}
			if providers, ok := llm["providers"].(map[string]interface{}); ok {
				for name, p := range providers {
					if pc, ok := p.(map[string]interface{}); ok {
						existing := s.cfg.LLM.Providers[name]
						if v, ok := pc["api_key"].(string); ok && v != "" && !strings.HasPrefix(v, "***") {
							existing.APIKey = v
							changedFields = append(changedFields, "llm.providers."+name+".api_key")
						}
						if v, ok := pc["api_url"].(string); ok {
							existing.APIURL = v
							changedFields = append(changedFields, "llm.providers."+name+".api_url")
						}
						if v, ok := pc["model"].(string); ok {
							existing.Model = v
							changedFields = append(changedFields, "llm.providers."+name+".model")
						}
						if v, ok := pc["max_tokens"].(float64); ok {
							existing.MaxTokens = int(v)
							changedFields = append(changedFields, "llm.providers."+name+".max_tokens")
						}
						if v, ok := pc["temperature"].(float64); ok {
							existing.Temperature = v
							changedFields = append(changedFields, "llm.providers."+name+".temperature")
						}
						if v, ok := pc["reasoning_effort"].(string); ok {
							existing.ReasoningEffort = v
							changedFields = append(changedFields, "llm.providers."+name+".reasoning_effort")
						}
						s.cfg.LLM.Providers[name] = existing
					}
				}
			}
		}

		// 更新 channels 配置
		if channels, ok := req["channels"].(map[string]interface{}); ok {
			if feishu, ok := channels["feishu"].(map[string]interface{}); ok {
				if v, ok := feishu["enabled"].(bool); ok {
					s.cfg.Channels.Feishu.Enabled = v
					changedFields = append(changedFields, "channels.feishu.enabled")
				}
				if v, ok := feishu["app_id"].(string); ok {
					s.cfg.Channels.Feishu.AppID = v
					changedFields = append(changedFields, "channels.feishu.app_id")
				}
				if v, ok := feishu["app_secret"].(string); ok && v != "" && !strings.HasPrefix(v, "***") {
					s.cfg.Channels.Feishu.AppSecret = v
					changedFields = append(changedFields, "channels.feishu.app_secret")
				}
			}
		}

		// 更新 tools 配置
		if tools, ok := req["tools"].(map[string]interface{}); ok {
			if shell, ok := tools["shell"].(map[string]interface{}); ok {
				if v, ok := shell["enabled"].(bool); ok {
					s.cfg.Tools.Shell.Enabled = v
					changedFields = append(changedFields, "tools.shell.enabled")
				}
			}
			if web, ok := tools["web"].(map[string]interface{}); ok {
				if v, ok := web["enabled"].(bool); ok {
					s.cfg.Tools.Web.Enabled = v
					changedFields = append(changedFields, "tools.web.enabled")
				}
			}
			if cron, ok := tools["cron"].(map[string]interface{}); ok {
				if v, ok := cron["enabled"].(bool); ok {
					s.cfg.Tools.Cron.Enabled = v
					changedFields = append(changedFields, "tools.cron.enabled")
				}
			}
		}

		// 审计日志：使用 RemoteAddr 防止 X-Forwarded-For 伪造
		clientIP := r.RemoteAddr
		if len(changedFields) > 0 {
			logger.Info("Config updated via API",
				logger.String("client_ip", clientIP),
				logger.String("changed_fields", strings.Join(changedFields, ",")),
				logger.Int("field_count", len(changedFields)),
			)
		} else {
			logger.Info("Config update request with no changes",
				logger.String("client_ip", clientIP),
			)
		}

		// 返回更新后的配置
		writeJSON(w,s.buildConfigResponse())

	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

