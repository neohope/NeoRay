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

// AnthropicProvider Anthropic API 提供商
type AnthropicProvider struct {
	name   string
	cfg    *config.ProviderConfig
	client *http.Client
}

// NewAnthropicProvider 创建 Anthropic 提供商
func NewAnthropicProvider(name string, cfg *config.ProviderConfig) *AnthropicProvider {
	return &AnthropicProvider{
		name: name,
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Name 提供商名称
func (p *AnthropicProvider) Name() string {
	return p.name
}

// anthropicRequest Anthropic API 请求
type anthropicRequest struct {
	Model         string                `json:"model"`
	Messages      []anthropicMessage    `json:"messages"`
	System        string                `json:"system,omitempty"`
	MaxTokens     int                   `json:"max_tokens"`
	Temperature   float64               `json:"temperature,omitempty"`
	Stream        bool                  `json:"stream,omitempty"`
	Tools         []anthropicTool       `json:"tools,omitempty"`
}

// anthropicMessage Anthropic 消息
type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

// anthropicContentBlock 内容块
type anthropicContentBlock struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result"

	// Text block
	Text string `json:"text,omitempty"`

	// Tool use block
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`

	// Tool result block
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`
}

// anthropicTool Anthropic 工具定义
type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicResponse Anthropic API 响应
type anthropicResponse struct {
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat 发送聊天请求
func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("%s api key not configured", p.name)
	}

	logger.Debug("Calling Anthropic API",
		logger.String("provider", p.name),
		logger.String("model", p.cfg.Model),
		logger.Int("tools_count", len(req.Tools)),
	)

	// 分离系统消息和普通消息
	var systemMessage string
	var apiMsgs []anthropicMessage

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemMessage = msg.Content
			continue
		}

		// 转换消息格式
		contentBlocks := make([]anthropicContentBlock, 0)

		if msg.Role == "tool" {
			// 工具响应 - 解析内容
			var toolResponses []map[string]interface{}
			if json.Unmarshal([]byte(msg.Content), &toolResponses) == nil {
				for _, tr := range toolResponses {
					if toolUseID, _ := tr["tool_use_id"].(string); toolUseID != "" {
						content, _ := tr["content"].(string)
						isError, _ := tr["is_error"].(bool)
						contentBlocks = append(contentBlocks, anthropicContentBlock{
							Type:      "tool_result",
							ToolUseID: toolUseID,
							Content:   content,
							IsError:   isError,
						})
					}
				}
			} else {
				// 简单工具响应
				contentBlocks = append(contentBlocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			// 工具响应用 user 角色
			apiMsgs = append(apiMsgs, anthropicMessage{
				Role:    "user",
				Content: contentBlocks,
			})
		} else if len(msg.ToolCalls) > 0 {
			// 带工具调用的消息 (assistant)
			for _, tc := range msg.ToolCalls {
				var input map[string]interface{}
				json.Unmarshal([]byte(tc.Arguments), &input)
				contentBlocks = append(contentBlocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			if msg.Content != "" {
				contentBlocks = append([]anthropicContentBlock{{
					Type: "text",
					Text: msg.Content,
				}}, contentBlocks...)
			}
			apiMsgs = append(apiMsgs, anthropicMessage{
				Role:    "assistant",
				Content: contentBlocks,
			})
		} else {
			// 普通文本消息
			contentBlocks = append(contentBlocks, anthropicContentBlock{
				Type: "text",
				Text: msg.Content,
			})
			apiMsgs = append(apiMsgs, anthropicMessage{
				Role:    msg.Role,
				Content: contentBlocks,
			})
		}
	}

	// 构建工具定义
	var apiTools []anthropicTool
	for _, tool := range req.Tools {
		logger.Debug("Adding tool to request",
			logger.String("name", tool.Name),
			logger.String("description", tool.Description))
		apiTools = append(apiTools, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema.(map[string]interface{}),
		})
	}

	// 构建请求
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.cfg.MaxTokens
	}
	temperature := req.Temperature
	if temperature == 0 {
		temperature = p.cfg.Temperature
	}

	apiReq := &anthropicRequest{
		Model:       p.cfg.Model,
		Messages:    apiMsgs,
		System:      systemMessage,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Tools:       apiTools,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.cfg.APIURL
	if apiURL == "" {
		apiURL = "https://api.anthropic.com"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s, body: %s", resp.Status, string(errBody))
	}

	// 读取原始响应
	respBody, _ := io.ReadAll(resp.Body)
	logger.Debug("Anthropic API raw response", logger.String("body", string(respBody)))

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 解析响应
	var content string
	var toolCalls []ToolCall

	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			inputBytes, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(inputBytes),
			})
		}
	}

	response := &ChatResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: apiResp.StopReason,
	}

	logger.Debug("Anthropic API response",
		logger.Int("input_tokens", apiResp.Usage.InputTokens),
		logger.Int("output_tokens", apiResp.Usage.OutputTokens),
		logger.Int("tool_calls", len(response.ToolCalls)),
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
			ToolCalls:    resp.ToolCalls,
			FinishReason: resp.FinishReason,
		}
	}()

	return resultChan, nil
}
