package channel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"neoray/internal/agent"
	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/session"
)

// FeishuConfig 飞书配置
type FeishuConfig struct {
	AppID           string
	AppSecret       string
	Enabled         bool
	VerificationToken string
	EncryptKey      string
	WebhookPath     string
}

// FeishuChannel 飞书频道
type FeishuChannel struct {
	cfg         *FeishuConfig
	appConfig   *config.Config
	agent       *agent.Agent
	sessionMgr  *session.Manager
	tenantToken string
	tokenExpiry time.Time
	httpClient  *http.Client
	mu          sync.RWMutex
	running     bool
	stopChan    chan struct{}
}

// NewFeishuChannel 创建飞书频道
func NewFeishuChannel(cfg *FeishuConfig, appConfig *config.Config, aiAgent *agent.Agent, sessionMgr *session.Manager) *FeishuChannel {
	return &FeishuChannel{
		cfg:        cfg,
		appConfig:  appConfig,
		agent:      aiAgent,
		sessionMgr: sessionMgr,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		stopChan:   make(chan struct{}),
	}
}

// Name 实现 Channel 接口
func (f *FeishuChannel) Name() string {
	return "feishu"
}

// Start 实现 Channel 接口
func (f *FeishuChannel) Start() error {
	if !f.cfg.Enabled {
		logger.Info("Feishu channel disabled")
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.running {
		return nil
	}

	logger.Info("Starting Feishu channel")

	// 获取 tenant access token
	if err := f.refreshToken(); err != nil {
		logger.Warn("Failed to get feishu token on startup", logger.ErrorField(err))
		// 不阻止启动，token 可以后续刷新
	}

	f.running = true

	// 启动 token 刷新 goroutine
	go f.tokenRefreshLoop()

	logger.Info("Feishu channel started")
	return nil
}

// Stop 实现 Channel 接口
func (f *FeishuChannel) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.running {
		return nil
	}

	logger.Info("Stopping Feishu channel")
	close(f.stopChan)
	f.running = false
	logger.Info("Feishu channel stopped")
	return nil
}

// SendMessage 实现 Channel 接口
func (f *FeishuChannel) SendMessage(ctx context.Context, receiveID, message string) error {
	f.mu.RLock()
	token := f.tenantToken
	f.mu.RUnlock()

	if token == "" {
		// 尝试刷新 token
		if err := f.refreshToken(); err != nil {
			return fmt.Errorf("no feishu token available")
		}
		token = f.tenantToken
	}

	// 构建消息请求
	msgBody := map[string]interface{}{
		"receive_id": receiveID,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, escapeJSON(message)),
		"uuid":       fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	body, _ := json.Marshal(msgBody)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=user_id",
		bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(respBody, &result)

	if resp.StatusCode != http.StatusOK || result.Code != 0 {
		return fmt.Errorf("feishu api error: %s (code: %d)", result.Msg, result.Code)
	}

	return nil
}

// HandleWebhook 处理飞书 Webhook 请求
func (f *FeishuChannel) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	logger.Debug("Received Feishu webhook", logger.String("body", string(body)))

	// 验证请求
	if f.cfg.VerificationToken != "" {
		token := r.Header.Get("X-Lark-Request-Token")
		if token != f.cfg.VerificationToken {
			logger.Warn("Invalid verification token", logger.String("token", token))
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
	}

	// 解析事件
	var event FeishuEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// URL 验证（首次配置时使用）
	if event.Type == "url_verification" {
		var challenge FeishuChallenge
		_ = json.Unmarshal(body, &challenge)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"challenge": challenge.Challenge,
		})
		return
	}

	// 处理消息事件
	if event.Header.EventType == "im.message.receive_v1" {
		go f.handleMessageEvent(body)
	}

	// 返回 200
	w.WriteHeader(http.StatusOK)
}

// handleMessageEvent 处理消息事件
func (f *FeishuChannel) handleMessageEvent(body []byte) {
	var msgEvent FeishuMessageEvent
	if err := json.Unmarshal(body, &msgEvent); err != nil {
		logger.Error("Failed to parse message event", logger.ErrorField(err))
		return
	}

	// 只处理文本消息
	if msgEvent.Event.Message.MessageType != "text" {
		logger.Debug("Ignoring non-text message",
			logger.String("type", msgEvent.Event.Message.MessageType))
		return
	}

	// 解析消息内容
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(msgEvent.Event.Message.Content), &content); err != nil {
		logger.Error("Failed to parse message content", logger.ErrorField(err))
		return
	}

	userID := msgEvent.Event.Sender.SenderID.OpenID
	message := content.Text

	logger.Info("Received Feishu message",
		logger.String("user_id", userID),
		logger.String("message", message))

	// 获取或创建会话（用 OpenID 作为会话标识）
	sessID := "feishu_" + userID
	sess, err := f.sessionMgr.GetSession(sessID)
	if err != nil {
		sess = session.NewSession()
		sess.ID = sessID
		sess.Title = "Feishu Chat"
		_ = f.sessionMgr.SaveSession(sess)
	}

	// 调用 Agent 处理
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := f.agent.Chat(ctx, sess, message)
	if err != nil {
		logger.Error("Agent chat failed", logger.ErrorField(err))
		_ = f.SendMessage(ctx, userID, "抱歉，处理失败："+err.Error())
		return
	}

	// 发送回复
	if err := f.SendMessage(ctx, userID, resp.Content); err != nil {
		logger.Error("Failed to send feishu message", logger.ErrorField(err))
	}
}

// refreshToken 刷新 tenant access token
func (f *FeishuChannel) refreshToken() error {
	reqBody := map[string]string{
		"app_id":     f.cfg.AppID,
		"app_secret": f.cfg.AppSecret,
	}

	body, _ := json.Marshal(reqBody)

	resp, err := f.httpClient.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("feishu token error: %s (code: %d)", result.Msg, result.Code)
	}

	f.mu.Lock()
	f.tenantToken = result.TenantAccessToken
	f.tokenExpiry = time.Now().Add(time.Duration(result.Expire) * time.Second)
	f.mu.Unlock()

	logger.Debug("Feishu token refreshed",
		logger.String("expiry", f.tokenExpiry.String()),
	)

	return nil
}

// tokenRefreshLoop 定期刷新 token
func (f *FeishuChannel) tokenRefreshLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查是否需要刷新（提前 10 分钟）
			f.mu.RLock()
			needsRefresh := f.tenantToken == "" || time.Now().Add(10*time.Minute).After(f.tokenExpiry)
			f.mu.RUnlock()

			if needsRefresh {
				if err := f.refreshToken(); err != nil {
					logger.Warn("Failed to refresh feishu token", logger.ErrorField(err))
				}
			}

		case <-f.stopChan:
			return
		}
	}
}

// FeishuEvent 飞书事件基础结构
type FeishuEvent struct {
	UUID   string `json:"uuid"`
	Token  string `json:"token"`
	TS     string `json:"ts"`
	Type   string `json:"type"`
	Header struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		CreateTime string `json:"create_time"`
		Token      string `json:"token"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
}

// FeishuChallenge URL 验证挑战
type FeishuChallenge struct {
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

// FeishuMessageEvent 消息事件
type FeishuMessageEvent struct {
	FeishuEvent
	Event struct {
		Sender struct {
			SenderID struct {
				UnionID string `json:"union_id"`
				UserID  string `json:"user_id"`
				OpenID  string `json:"open_id"`
			} `json:"sender_id"`
			SenderType string `json:"sender_type"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
		} `json:"message"`
	} `json:"event"`
}

// escapeJSON 转义 JSON 字符串
func escapeJSON(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// generateHmac 生成 HMAC 签名
func generateHmac(data, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
