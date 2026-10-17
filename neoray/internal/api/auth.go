package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"neoray/internal/logger"
)

// webSessionCookieName 是 Web UI 会话 Cookie 名称。
const webSessionCookieName = "neoray_session"

// webSessionDefaultTTL 是会话默认有效期（24 小时）。
const webSessionDefaultTTL = 24 * time.Hour

// webSessionStore 是进程内的 Web UI 会话存储。
//
// 为什么不落盘：Web UI 面向本机使用，会话仅需在进程生命周期内有效。
// 内存存储避免了额外的数据库依赖与令牌泄漏面。
type webSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token -> 过期时间
	stopCh   chan struct{}
}

// newWebSessionStore 创建会话存储并启动过期清理协程。
func newWebSessionStore() *webSessionStore {
	s := &webSessionStore{
		sessions: make(map[string]time.Time),
		stopCh:   make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// cleanupLoop 周期性清理过期会话，防止内存无界增长。
func (s *webSessionStore) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for token, exp := range s.sessions {
				if now.After(exp) {
					delete(s.sessions, token)
				}
			}
			s.mu.Unlock()
		}
	}
}

// Create 生成新会话令牌。
func (s *webSessionStore) Create(ttl time.Duration) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)

	if ttl <= 0 {
		ttl = webSessionDefaultTTL
	}
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(ttl)
	s.mu.Unlock()
	return token, nil
}

// Validate 校验令牌是否有效且未过期。
func (s *webSessionStore) Validate(token string) bool {
	if token == "" {
		return false
	}
	s.mu.RLock()
	exp, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		s.Delete(token)
		return false
	}
	return true
}

// Delete 删除指定令牌（登出）。
func (s *webSessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// stop 停止清理协程。
func (s *webSessionStore) stop() {
	close(s.stopCh)
}

// validWebSession 检查请求是否携带有效的 Web UI 会话 Cookie。
//
// 背景：浏览器的 WebSocket API 无法自定义请求头，因此 Web UI 无法使用
// X-Api-Key 方式鉴权。改用同源 Cookie —— 浏览器在 WS 握手时会自动携带。
func (s *Server) validWebSession(r *http.Request) bool {
	if s.webSessions == nil {
		return false
	}
	c, err := r.Cookie(webSessionCookieName)
	if err != nil {
		return false
	}
	return s.webSessions.Validate(c.Value)
}

// handleAuthLogin 处理登录：校验密钥并下发会话 Cookie。
//
// 请求体：{"key":"<secret_key>"}
// 响应：{"ok":true,"expires_in":86400}
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg := s.getConfig()

	// 鉴权未启用时无需登录，直接放行以便前端继续。
	if !cfg.Security.Auth.Enabled {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{"ok": true, "auth_disabled": true})
		return
	}

	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	expected := cfg.Security.Auth.SecretKey
	if expected == "" {
		// 与 authenticateRequest 保持一致：启用鉴权但未设密钥时拒绝所有请求。
		writeJSONError(w, http.StatusUnauthorized, "server auth key is not configured")
		return
	}

	if subtle.ConstantTimeCompare([]byte(body.Key), []byte(expected)) != 1 {
		logger.Warn("Web UI login failed: invalid key",
			logger.String("remote", r.RemoteAddr))
		writeJSONError(w, http.StatusUnauthorized, "invalid key")
		return
	}

	ttl := cfg.Security.Auth.JWTExpiry
	if ttl <= 0 {
		ttl = webSessionDefaultTTL
	}

	token, err := s.webSessions.Create(ttl)
	if err != nil {
		logger.Error("Failed to create web session", logger.ErrorField(err))
		writeJSONError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true, // 禁止 JS 读取，降低 XSS 窃取风险
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil, // 仅在 HTTPS 下要求 Secure
		MaxAge:   int(ttl.Seconds()),
	})

	logger.Info("Web UI login succeeded", logger.String("remote", r.RemoteAddr))

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]interface{}{
		"ok":         true,
		"expires_in": int(ttl.Seconds()),
	})
}

// handleAuthLogout 处理登出：销毁会话并清除 Cookie。
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if c, err := r.Cookie(webSessionCookieName); err == nil {
		s.webSessions.Delete(c.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]interface{}{"ok": true})
}

// handleAuthMe 返回当前会话状态，供前端判断是否已登录。
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg := s.getConfig()
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]interface{}{
		"authenticated": s.authenticateRequest(r),
		"auth_enabled":  cfg.Security.Auth.Enabled,
	})
}
