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

// OpenAIProvider OpenAI 兼容提供商
type OpenAIProvider struct {
	cfg    *config.OpenAIConfig
	client *http.Client
}

// NewOpenAIProvider 创建 OpenAI 提供商
func NewOpenAIProvider(cfg *config.OpenAIConfig) *OpenAIProvider {
	return &OpenAIProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name 提供商名称
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// openaiRequest OpenAI API 请求
type openaiRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	MaxTokens  int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream     bool      `json:"stream,omitempty"`
}

// openaiResponse OpenAI API 响应
type openaiResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Chat 发送聊天请求
func (p *OpenAIProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("openai api key not configured")
	}

	logger.Debug("Calling OpenAI compatible API", logger.String("model", p.cfg.Model))

	// 构建请求
	apiReq := &openaiRequest{
		Model:       p.cfg.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = p.cfg.MaxTokens
	}
	if apiReq.Temperature == 0 {
		apiReq.Temperature = p.cfg.Temperature
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.cfg.APIURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s, body: %s", resp.Status, string(errBody))
	}

	var apiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := apiResp.Choices[0]
	response := &ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
	}

	logger.Debug("OpenAI API response",
		logger.Int("prompt_tokens", apiResp.Usage.PromptTokens),
		logger.Int("completion_tokens", apiResp.Usage.CompletionTokens),
	)

	return response, nil
}

// ChatStream 流式聊天
func (p *OpenAIProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error) {
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
			ToolCalls:    resp.ToolCalls,
			FinishReason: resp.FinishReason,
		}
	}()

	return resultChan, nil
}
