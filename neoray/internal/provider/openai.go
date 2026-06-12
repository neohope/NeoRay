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
	Model       string               `json:"model"`
	Messages    []openaiMessage      `json:"messages"`
	Tools       []openaiTool         `json:"tools,omitempty"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
}

// openaiTool OpenAI 工具定义
type openaiTool struct {
	Type     string                 `json:"type"`
	Function openaiFunction         `json:"function"`
}

// openaiFunction 工具函数定义
type openaiFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// openaiMessage OpenAI 消息
type openaiMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCalls  []openaiToolCallItem `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	Name       string               `json:"name,omitempty"`
}

// openaiToolCallItem OpenAI 工具调用项
type openaiToolCallItem struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openaiToolCallFunction `json:"function"`
}

// openaiToolCallFunction OpenAI 工具调用函数
type openaiToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openaiResponse OpenAI API 响应
type openaiResponse struct {
	Choices []struct {
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
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

	logger.Debug("Calling OpenAI compatible API",
		logger.String("model", p.cfg.Model),
		logger.Int("tools_count", len(req.Tools)),
	)

	// 转换消息格式
	apiMsgs := make([]openaiMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			// 解析工具响应
			var toolResponses []map[string]interface{}
			if json.Unmarshal([]byte(msg.Content), &toolResponses) == nil {
				for _, tr := range toolResponses {
					if toolUseID, _ := tr["tool_use_id"].(string); toolUseID != "" {
						content, _ := tr["content"].(string)
						apiMsgs = append(apiMsgs, openaiMessage{
							Role:       "tool",
							Content:    content,
							ToolCallID: toolUseID,
						})
					}
				}
			} else {
				// 不是工具响应数组，直接添加
				apiMsgs = append(apiMsgs, openaiMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		} else {
			// 转换 tool calls 格式
			var oaiToolCalls []openaiToolCallItem
			for _, tc := range msg.ToolCalls {
				oaiToolCalls = append(oaiToolCalls, openaiToolCallItem{
					ID:   tc.ID,
					Type: "function",
					Function: openaiToolCallFunction{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
			apiMsgs = append(apiMsgs, openaiMessage{
				Role:       msg.Role,
				Content:    msg.Content,
				ToolCalls:  oaiToolCalls,
			})
		}
	}

	// 构建工具定义
	var apiTools []openaiTool
	for _, tool := range req.Tools {
		logger.Debug("Adding tool to OpenAI request",
			logger.String("name", tool.Name),
			logger.String("description", tool.Description))
		apiTools = append(apiTools, openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema.(map[string]interface{}),
			},
		})
	}

	// 构建请求
	apiReq := &openaiRequest{
		Model:       p.cfg.Model,
		Messages:    apiMsgs,
		Tools:       apiTools,
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

	// 先读取原始响应用于调试
	respBody, _ := io.ReadAll(resp.Body)
	logger.Debug("OpenAI raw response", logger.String("body", string(respBody)))

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := apiResp.Choices[0]

	// 调试 tool calls
	logger.Debug("OpenAI message",
		logger.String("content", choice.Message.Content),
		logger.Int("tool_calls_count", len(choice.Message.ToolCalls)))

	// 转换 tool calls 回通用格式
	var toolCalls []ToolCall
	for _, tc := range choice.Message.ToolCalls {
		logger.Debug("OpenAI tool call",
			logger.String("id", tc.ID),
			logger.String("name", tc.Function.Name),
			logger.String("arguments", tc.Function.Arguments))
		toolCalls = append(toolCalls, ToolCall{
			ID:       tc.ID,
			Name:     tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	response := &ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    toolCalls,
		FinishReason: choice.FinishReason,
	}

	logger.Debug("OpenAI API response",
		logger.Int("prompt_tokens", apiResp.Usage.PromptTokens),
		logger.Int("completion_tokens", apiResp.Usage.CompletionTokens),
		logger.Int("tool_calls", len(response.ToolCalls)),
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
