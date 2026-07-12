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

// GenericProvider 通用 OpenAI 兼容提供商
type GenericProvider struct {
	name      string
	cfg       *config.ProviderConfig
	client    *http.Client
	generation GenerationSettings
}

// NewGenericProvider 创建通用提供商
func NewGenericProvider(name string, cfg *config.ProviderConfig) *GenericProvider {
	return &GenericProvider{
		name: name,
		cfg:  cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		generation: GenerationSettings{
			Temperature:      cfg.Temperature,
			MaxTokens:        cfg.MaxTokens,
			ReasoningEffort: cfg.ReasoningEffort,
		},
	}
}

// Name 提供商名称
func (p *GenericProvider) Name() string {
	return p.name
}

// GetGenerationSettings 获取生成设置
func (p *GenericProvider) GetGenerationSettings() GenerationSettings {
	return p.generation
}

// SetGenerationSettings 设置生成设置
func (p *GenericProvider) SetGenerationSettings(settings GenerationSettings) {
	p.generation = settings
}

// GetDefaultModel 获取默认模型
func (p *GenericProvider) GetDefaultModel() string {
	return p.cfg.Model
}

// openaiRequest OpenAI API 请求
type openaiRequest struct {
	Model             string               `json:"model"`
	Messages          []openaiMessage      `json:"messages"`
	Tools             []openaiTool         `json:"tools,omitempty"`
	MaxTokens         int                  `json:"max_tokens,omitempty"`
	Temperature       float64              `json:"temperature,omitempty"`
	Stream            bool                 `json:"stream,omitempty"`
	ReasoningEffort  string               `json:"reasoning_effort,omitempty"` // 一些 OpenAI 兼容 API 支持
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
		Index        int           `json:"index"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		// 一些 OpenAI 兼容 API 返回缓存相关字段
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage"`
}

// openaiStreamResponse 流式响应
type openaiStreamResponse struct {
	Choices []struct {
		Delta struct {
			Role      string               `json:"role"`
			Content   string               `json:"content"`
			ToolCalls []openaiToolCallItem `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
}

// Chat 发送聊天请求
func (p *GenericProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("%s api key not configured", p.name)
	}

	logger.Debug("Calling API",
		logger.String("provider", p.name),
		logger.String("model", p.cfg.Model),
		logger.Int("tools_count", len(req.Tools)),
		logger.String("reasoning_effort", req.ReasoningEffort),
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
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: oaiToolCalls,
			})
		}
	}

	// 构建工具定义
	var apiTools []openaiTool
	for _, tool := range req.Tools {
		logger.Debug("Adding tool to request",
			logger.String("name", tool.Name),
			logger.String("description", tool.Description))
		schema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			logger.Warn("Skipping tool with invalid InputSchema", logger.String("tool", tool.Name))
			continue
		}
		apiTools = append(apiTools, openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  schema,
			},
		})
	}

	// 构建请求
	apiReq := &openaiRequest{
		Model:           p.cfg.Model,
		Messages:        apiMsgs,
		Tools:           apiTools,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
	}

	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = p.generation.MaxTokens
		if apiReq.MaxTokens == 0 {
			apiReq.MaxTokens = p.cfg.MaxTokens
		}
	}
	if apiReq.Temperature == 0 {
		apiReq.Temperature = p.generation.Temperature
		if apiReq.Temperature == 0 {
			apiReq.Temperature = p.cfg.Temperature
		}
	}
	if apiReq.ReasoningEffort == "" {
		apiReq.ReasoningEffort = p.generation.ReasoningEffort
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
		return p.handleError(err), fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		errResp := p.parseErrorResponse(errBody, resp.StatusCode)
		return errResp, fmt.Errorf("api error: %s, body: %s", resp.Status, string(errBody))
	}

	// 先读取原始响应用于调试
	respBody, _ := io.ReadAll(resp.Body)
	logger.Debug("API raw response", logger.String("body", string(respBody)))

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := apiResp.Choices[0]

	// 转换 tool calls 回通用格式
	var toolCalls []ToolCall
	for _, tc := range choice.Message.ToolCalls {
		logger.Debug("Tool call",
			logger.String("id", tc.ID),
			logger.String("name", tc.Function.Name),
			logger.String("arguments", tc.Function.Arguments))
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	// 构建使用量
	usage := &Usage{
		InputTokens:            apiResp.Usage.PromptTokens,
		OutputTokens:           apiResp.Usage.CompletionTokens,
		CacheCreationInputTokens: apiResp.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     apiResp.Usage.CacheReadInputTokens,
	}
	if apiResp.Usage.CacheReadInputTokens > 0 {
		usage.CachedTokens = apiResp.Usage.CacheReadInputTokens
	}

	response := &ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    toolCalls,
		FinishReason: choice.FinishReason,
		Usage:       usage,
	}

	logger.Debug("API response",
		logger.Int("prompt_tokens", apiResp.Usage.PromptTokens),
		logger.Int("completion_tokens", apiResp.Usage.CompletionTokens),
		logger.Int("cache_creation_tokens", apiResp.Usage.CacheCreationInputTokens),
		logger.Int("cache_read_tokens", apiResp.Usage.CacheReadInputTokens),
		logger.Int("tool_calls", len(response.ToolCalls)),
	)

	return response, nil
}

// ChatStream 流式聊天
func (p *GenericProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("%s api key not configured", p.name)
	}

	resultChan := make(chan StreamChatResponse)

	// 先尝试用非流式实现
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

// handleError 处理错误
func (p *GenericProvider) handleError(err error) *ChatResponse {
	errResp := &ChatResponse{
		Content:       fmt.Sprintf("Error calling LLM: %v", err),
		FinishReason: "error",
	}

	// 检查错误类型
	errMsg := err.Error()
	if containsAny(errMsg, "timeout") {
		errResp.ErrorType = "timeout"
		errResp.ErrorShouldRetry = true
	} else if containsAny(errMsg, "connection") {
		errResp.ErrorType = "connection"
		errResp.ErrorShouldRetry = true
	}

	return errResp
}

// parseErrorResponse 解析错误响应
func (p *GenericProvider) parseErrorResponse(body []byte, statusCode int) *ChatResponse {
	errResp := &ChatResponse{
		FinishReason:    "error",
		ErrorStatusCode: statusCode,
	}

	var errStruct struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    string `json:"code,omitempty"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errStruct); err == nil {
		errResp.Content = errStruct.Error.Message
		errResp.ErrorType = errStruct.Error.Type
		errResp.ErrorCode = errStruct.Error.Code

		// 判断是否应该重试
		if statusCode >= 500 || statusCode == 408 || statusCode == 429 {
			errResp.ErrorShouldRetry = true
		}

		// 检查是否是欠费/配额错误（非重试）
		arrearageTokens := []string{
			"insufficient_quota", "quota_exceeded", "quota_exhausted",
			"billing_hard_limit", "insufficient_balance", "payment_required",
		}
		for _, token := range arrearageTokens {
			if containsAny(errStruct.Error.Type, token) || containsAny(errStruct.Error.Message, token) {
				errResp.ErrorShouldRetry = false
			}
		}
	} else {
		errResp.Content = string(body)
	}

	return errResp
}
