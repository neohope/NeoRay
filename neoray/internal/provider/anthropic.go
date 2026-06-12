package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// AnthropicProvider Anthropic Claude 提供商
type AnthropicProvider struct {
	cfg    *config.AnthropicConfig
	client *http.Client
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
	Tools      []anthropicTool    `json:"tools,omitempty"`
	Stream     bool               `json:"stream,omitempty"`
}

// anthropicTool Anthropic 工具定义
type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicMessage Anthropic 消息
type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // 可以是 string 或 []anthropicContentBlock
}

// anthropicContentBlock 内容块
type anthropicContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input string `json:"input,omitempty"`
}

// anthropicResponse Anthropic API 响应
type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	StopReason string `json:"stop_reason"`
}

// anthropicContent Anthropic 内容
type anthropicContent struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

// anthropicStreamEvent Anthropic 流式事件
type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`
	Usage json.RawMessage `json:"usage,omitempty"`
	Block json.RawMessage `json:"block,omitempty"`
}

// anthropicTextDelta 文本 Delta
type anthropicTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicToolUseBlock 工具使用块
type anthropicToolUseBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// Chat 发送聊天请求
func (p *AnthropicProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic api key not configured")
	}

	logger.Debug("Calling Anthropic API",
		logger.String("model", p.cfg.Model),
		logger.Int("tools_count", len(req.Tools)),
	)

	// 构建请求
	apiReq, systemMsg := p.buildRequest(req)
	apiReq.Stream = false

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

	// 先读取原始响应用于调试
	respBody, _ := io.ReadAll(resp.Body)
	logger.Debug("Anthropic raw response", logger.String("body", string(respBody)))

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 构建响应
	response := &ChatResponse{
		Content:      "",
		ToolCalls:    []ToolCall{},
		FinishReason: apiResp.StopReason,
	}

	for _, c := range apiResp.Content {
		logger.Debug("Anthropic content block",
			logger.String("type", c.Type),
			logger.String("name", c.Name),
			logger.String("id", c.ID))
		if c.Type == "text" {
			response.Content += c.Text
		} else if c.Type == "tool_use" {
			// 序列化 input 为字符串
			inputBytes, _ := json.Marshal(c.Input)
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				ID:        c.ID,
				Name:      c.Name,
				Arguments: string(inputBytes),
			})
			logger.Debug("Added tool call",
				logger.String("id", c.ID),
				logger.String("name", c.Name),
				logger.String("args", string(inputBytes)))
		}
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
	resultChan := make(chan StreamChatResponse, 100)

	if p.cfg.APIKey == "" {
		close(resultChan)
		return resultChan, fmt.Errorf("anthropic api key not configured")
	}

	// 构建请求
	apiReq, systemMsg := p.buildRequest(req)
	apiReq.Stream = true

	body, err := json.Marshal(apiReq)
	if err != nil {
		close(resultChan)
		return resultChan, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.cfg.APIURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		close(resultChan)
		return resultChan, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")
	if systemMsg != "" {
		httpReq.Header.Set("anthropic-dangerous-direct-browser-access", "true")
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		close(resultChan)
		return resultChan, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		close(resultChan)
		return resultChan, fmt.Errorf("api error: %s, body: %s", resp.Status, string(errBody))
	}

	go func() {
		defer close(resultChan)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var currentToolCalls []ToolCall
		var pendingToolUse *ToolCall
		var pendingInputBuffer strings.Builder

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				resultChan <- StreamChatResponse{Error: ctx.Err()}
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				continue
			}

			// 解析 SSE 格式
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				var event anthropicStreamEvent
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					logger.Debug("Failed to unmarshal stream event", logger.String("data", data), logger.ErrorField(err))
					continue
				}

				switch event.Type {
				case "message_start":
					logger.Debug("Stream message_start")
				case "content_block_start":
					// 内容块开始
					var block anthropicContent
					if err := json.Unmarshal(event.Block, &block); err == nil {
						if block.Type == "tool_use" {
							// 工具调用开始
							inputBytes, _ := json.Marshal(block.Input)
							pendingToolUse = &ToolCall{
								ID:        block.ID,
								Name:      block.Name,
								Arguments: string(inputBytes),
							}
							pendingInputBuffer.Reset()
							// 如果已经有 input，先保存
							if len(inputBytes) > 0 && string(inputBytes) != "{}" {
								pendingInputBuffer.Write(inputBytes)
							}
						}
					}
				case "content_block_delta":
					// 内容 delta
					var delta anthropicTextDelta
					if err := json.Unmarshal(event.Delta, &delta); err == nil {
						if delta.Type == "text_delta" {
							resultChan <- StreamChatResponse{
								Content: delta.Text,
							}
						} else if delta.Type == "input_json_delta" && pendingToolUse != nil {
							// 工具输入 delta（如果有的话）
							var inputDelta struct {
								PartialJSON string `json:"partial_json"`
							}
							if json.Unmarshal(event.Delta, &inputDelta) == nil {
								pendingInputBuffer.WriteString(inputDelta.PartialJSON)
							}
						}
					}
				case "content_block_stop":
					// 内容块结束
					if pendingToolUse != nil {
						// 完成工具调用
						if pendingInputBuffer.Len() > 0 {
							// 尝试解析完整的 JSON
							var input map[string]interface{}
							if json.Unmarshal([]byte(pendingInputBuffer.String()), &input) == nil {
								inputBytes, _ := json.Marshal(input)
								pendingToolUse.Arguments = string(inputBytes)
							} else {
								// 如果解析失败，直接使用缓冲区内容
								pendingToolUse.Arguments = pendingInputBuffer.String()
							}
						}
						currentToolCalls = append(currentToolCalls, *pendingToolUse)
						pendingToolUse = nil
					}
				case "message_delta":
					// 消息 delta（可能包含 finish_reason）
					var delta struct {
						StopReason string `json:"stop_reason"`
					}
					if json.Unmarshal(event.Delta, &delta) == nil {
						resultChan <- StreamChatResponse{
							FinishReason: delta.StopReason,
							ToolCalls:    currentToolCalls,
						}
					}
				case "message_stop":
					logger.Debug("Stream message_stop")
					return
				case "error":
					logger.Error("Stream error event", logger.String("data", data))
					var errResp struct {
						Error struct {
							Message string `json:"message"`
						} `json:"error"`
					}
					if json.Unmarshal([]byte(data), &errResp) == nil {
						resultChan <- StreamChatResponse{
							Error: fmt.Errorf("anthropic error: %s", errResp.Error.Message),
						}
					}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			logger.Error("Stream scanner error", logger.ErrorField(err))
			resultChan <- StreamChatResponse{Error: err}
		}
	}()

	return resultChan, nil
}

// ChatStreamWithTools 流式聊天（带工具调用支持）
func (p *AnthropicProvider) ChatStreamWithTools(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error) {
	// 复用 ChatStream，因为它已经支持工具调用了
	return p.ChatStream(ctx, req)
}

// buildRequest 构建请求
func (p *AnthropicProvider) buildRequest(req *ChatRequest) (*anthropicRequest, string) {
	apiReq := &anthropicRequest{
		Model:       p.cfg.Model,
		Messages:    make([]anthropicMessage, 0, len(req.Messages)),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	var systemMsg string

	// 添加工具定义
	if len(req.Tools) > 0 {
		apiReq.Tools = make([]anthropicTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			apiReq.Tools = append(apiReq.Tools, anthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: tool.InputSchema.(map[string]interface{}),
			})
		}
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemMsg = msg.Content
			continue
		}

		role := msg.Role
		if role == "tool" {
			role = "user"
		}

		// 处理内容
		var content interface{}

		if len(msg.ToolCalls) > 0 {
			// 有工具调用，使用内容块格式
			blocks := []anthropicContentBlock{}
			if msg.Content != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Arguments,
				})
			}
			content = blocks
		} else if role == "user" && msg.Content != "" && msg.Content[0] == '[' {
			// 检查是否是工具响应（JSON 数组）
			var toolResponses []map[string]interface{}
			if json.Unmarshal([]byte(msg.Content), &toolResponses) == nil {
				// 是工具响应，使用内容块格式
				blocks := []anthropicContentBlock{}
				for _, tr := range toolResponses {
					if toolUseID, ok := tr["tool_use_id"].(string); ok {
						if content, ok := tr["content"].(string); ok {
							blocks = append(blocks, anthropicContentBlock{
								Type: "tool_result",
								Text: content,
								ID:   toolUseID,
							})
						}
					}
				}
				if len(blocks) > 0 {
					content = blocks
				} else {
					content = msg.Content
				}
			} else {
				content = msg.Content
			}
		} else {
			content = msg.Content
		}

		apiReq.Messages = append(apiReq.Messages, anthropicMessage{
			Role:    role,
			Content: content,
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
