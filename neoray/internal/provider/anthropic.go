package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// AnthropicProvider Anthropic Claude 提供商
type AnthropicProvider struct {
	cfg     *config.AnthropicConfig
	client  *http.Client
}

// NewAnthropicProvider 创建 Anthropic 提供商
func NewAnthropicProvider(cfg *config.AnthropicConfig) *AnthropicProvider {
	return &AnthropicProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name 提供商名称
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// anthropicRequest Anthropic API 请求
type anthropicRequest struct {
	Model      string             `json:"model"`
	Messages   []anthropicMessage `json:"messages"`
	MaxTokens  int                `json:"max_tokens"`
	Temperature float64           `json:"temperature,omitempty"`
	System     string             `json:"system,omitempty"`
}

// anthropicMessage Anthropic 消息
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse Anthropic API 响应
type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage  struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	StopReason string `json:"stop_reason"`
}

// anthropicContent Anthropic 内容
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Chat 发送聊天请求
func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic api key not configured")
	}

	logger.Debug("Calling Anthropic API", logger.String("model", p.cfg.Model))

	// 构建请求
	apiReq, systemMsg := p.buildRequest(req)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.cfg.APIURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	if systemMsg != "" {
		httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s, body: %s", resp.Status, string(errBody))
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 构建响应
	response := &ChatResponse{
		Content:      "",
		FinishReason: apiResp.StopReason,
	}
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			response.Content += c.Text
		}
	}

	logger.Debug("Anthropic API response",
		logger.Int("input_tokens", apiResp.Usage.InputTokens),
		logger.Int("output_tokens", apiResp.Usage.OutputTokens),
	)

	return response, nil
}

// ChatStream 流式聊天
func (p *AnthropicProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error) {
	// 暂时使用非流式实现
	resultChan := make(chan StreamChatResponse, 1)

	go func() {
		defer close(resultChan)
		resp, err := p.Chat(ctx, req)
		if err != nil {
			resultChan <- StreamChatResponse{Error: err}
			return
		}
		resultChan <- StreamChatResponse{
			Content:      resp.Content,
			FinishReason: resp.FinishReason,
		}
	}()

	return resultChan, nil
}

// buildRequest 构建请求
func (p *AnthropicProvider) buildRequest(req *ChatRequest) (*anthropicRequest, string) {
	apiReq := &anthropicRequest{
		Model:      p.cfg.Model,
		Messages:   make([]anthropicMessage, 0, len(req.Messages)),
		MaxTokens:  req.MaxTokens,
		Temperature: req.Temperature,
	}

	var systemMsg string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
			continue
		}
		role := msg.Role
		if role == "tool" {
			role = "user"
		}
		apiReq.Messages = append(apiReq.Messages, anthropicMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	apiReq.System = systemMsg

	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = p.cfg.MaxTokens
	}
	if apiReq.Temperature == 0 {
		apiReq.Temperature = p.cfg.Temperature
	}

	return apiReq, systemMsg
}
