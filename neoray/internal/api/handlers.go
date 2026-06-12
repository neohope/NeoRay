package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"neoray/internal/logger"
	"neoray/internal/session"
)

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

	// 获取或创建会话
	var sess *session.Session
	var err error

	if chatPayload.SessionID != "" {
		sess, err = c.Server.sessionMgr.GetSession(chatPayload.SessionID)
		if err != nil {
			sess, _ = c.Server.sessionMgr.CreateSession()
		}
	} else {
		sess, _ = c.Server.sessionMgr.CreateSession()
	}

	c.SessionID = sess.ID

	// 发送开始响应
	c.sendMessage("chat_start", map[string]interface{}{
		"session_id": sess.ID,
	})

	// 调用 Agent 处理（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	resp, err := c.Server.agent.Chat(ctx, sess, chatPayload.Message)
	if err != nil {
		logger.Error("Agent chat failed", logger.ErrorField(err))
		c.sendError("agent_error", err.Error())
		return
	}

	// 发送最终响应
	c.sendMessage("chat_end", map[string]interface{}{
		"session_id": sess.ID,
		"content":    resp.Content,
	})
}

// handleCreateSession 处理创建会话
func (c *Client) handleCreateSession(payload interface{}) {
	var createPayload CreateSessionPayload
	payloadBytes, _ := json.Marshal(payload)
	if err := json.Unmarshal(payloadBytes, &createPayload); err != nil {
		c.sendError("invalid_payload", "Invalid create session payload")
		return
	}

	title := createPayload.Name
	if title == "" {
		title = "New Session"
	}

	sess, _ := c.Server.sessionMgr.CreateSession()
	if title != "New Session" {
		sess.Title = title
		_ = c.Server.sessionMgr.SaveSession(sess)
	}
	c.SessionID = sess.ID

	c.sendMessage("session_created", map[string]interface{}{
		"session_id": sess.ID,
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

	sess, err := c.Server.sessionMgr.GetSession(joinPayload.SessionID)
	if err != nil {
		c.sendError("session_not_found", "Session not found")
		return
	}

	c.SessionID = sess.ID

	c.sendMessage("session_joined", map[string]interface{}{
		"session_id": sess.ID,
		"name":       sess.Title,
		"messages":   sess.Messages,
	})
}

// handleListSessions 处理列出会话
func (c *Client) handleListSessions() {
	sessions, err := c.Server.sessionMgr.ListSessions()
	if err != nil {
		c.sendError("list_error", "Failed to list sessions")
		return
	}

	// 转换为简化格式
	sessionList := make([]map[string]interface{}, 0, len(sessions))
	for _, sess := range sessions {
		sessionList = append(sessionList, map[string]interface{}{
			"id":            sess.ID,
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

	switch r.Method {
	case http.MethodGet:
		// 列出会话
		sessions, err := s.sessionMgr.ListSessions()
		if err != nil {
			http.Error(w, `{"error":"Failed to list sessions"}`, http.StatusInternalServerError)
			return
		}

		sessionList := make([]map[string]interface{}, 0, len(sessions))
		for _, sess := range sessions {
			sessionList = append(sessionList, map[string]interface{}{
				"id":            sess.ID,
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
		// 创建会话
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		title := req.Name
		if title == "" {
			title = "New Session"
		}

		sess, _ := s.sessionMgr.CreateSession()
		if title != "New Session" {
			sess.Title = title
			_ = s.sessionMgr.SaveSession(sess)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         sess.ID,
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
		sess, err := s.sessionMgr.GetSession(sessionID)
		if err != nil {
			http.Error(w, `{"error":"Session not found"}`, http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(sess)

	case http.MethodPost:
		// 发送聊天消息
		var req struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, `{"error":"Message cannot be empty"}`, http.StatusBadRequest)
			return
		}

		sess, err := s.sessionMgr.GetSession(sessionID)
		if err != nil {
			http.Error(w, `{"error":"Session not found"}`, http.StatusNotFound)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		resp, err := s.agent.Chat(ctx, sess, req.Message)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": resp.Content,
		})

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
