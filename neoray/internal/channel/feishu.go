package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// FeishuConfig 飞书配置
type FeishuConfig struct {
	AppID     string `toml:"app_id"`
	AppSecret string `toml:"app_secret"`
	Enabled   bool   `toml:"enabled"`
}

// FeishuChannel 飞书频道
type FeishuChannel struct {
	cfg         *FeishuConfig
	appConfig   *config.Config
	tenantToken string
	tokenExpiry time.Time
	httpClient  *http.Client
	mu          sync.RWMutex
	running     bool
	stopChan    chan struct{}
}

// NewFeishuChannel 创建飞书频道
func NewFeishuChannel(cfg *FeishuConfig, appConfig *config.Config) *FeishuChannel {
	return &FeishuChannel{
		cfg:        cfg,
		appConfig:  appConfig,
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
		return fmt.Errorf("failed to get feishu token: %w", err)
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
		return fmt.Errorf("no feishu token available")
	}

	// 构建消息请求
	msgBody := map[string]interface{}{
		"receive_id": receiveID,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, escapeJSON(message)),
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feishu api error: %s", resp.Status)
	}

	return nil
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
		return fmt.Errorf("feishu token error: %s", result.Msg)
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
			needsRefresh := time.Now().Add(10 * time.Minute).After(f.tokenExpiry)
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
