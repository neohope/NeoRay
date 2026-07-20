package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// GenericProvider 通用 OpenAI 兼容提供商
type GenericProvider struct {
	name      string
	cfg       *config.ProviderConfig
	client    *http.Client
	streamClient *http.Client // 无超时，用于 SSE 流式请求

	mu         sync.RWMutex
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
		// 流式客户端无超时，依赖 context 取消控制生命周期
		streamClient: &http.Client{
			Timeout: 0,
		},
		generation: GenerationSettings{
			Temperature:     cfg.Temperature,
			MaxTokens:       cfg.MaxTokens,
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
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.generation
}

// SetGenerationSettings 设置生成设置
func (p *GenericProvider) SetGenerationSettings(settings GenerationSettings) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.generation = settings
}

// getGenerationCopy 返回 generation 的副本
func (p *GenericProvider) getGenerationCopy() GenerationSettings {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.generation
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

	gen := p.getGenerationCopy()
	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = gen.MaxTokens
		if apiReq.MaxTokens == 0 {
			apiReq.MaxTokens = p.cfg.MaxTokens
		}
	}
	if apiReq.Temperature == 0 {
		apiReq.Temperature = gen.Temperature
		if apiReq.Temperature == 0 {
			apiReq.Temperature = p.cfg.Temperature
		}
	}
	if apiReq.ReasoningEffort == "" {
		apiReq.ReasoningEffort = gen.ReasoningEffort
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

	// 调试日志：显示 API key 的前缀和长度（不显示完整 key）
	keyLen := len(p.cfg.APIKey)
	keyPrefix := ""
	if keyLen > 6 {
		keyPrefix = p.cfg.APIKey[:6] + "..."
	} else if keyLen > 0 {
		keyPrefix = "(short key)"
	} else {
		keyPrefix = "(empty)"
	}
	logger.Debug("API request",
		logger.String("url", p.cfg.APIURL),
		logger.String("key_prefix", keyPrefix),
		logger.Int("key_length", keyLen))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return p.handleError(err), fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // 64 KB max for error bodies
		errResp := p.parseErrorResponse(errBody, resp.StatusCode)
		return errResp, fmt.Errorf("api error: %s", resp.Status)
	}

	// 读取响应体（限制 10 MB，防止恶意响应耗尽内存）
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	logger.Debug("API response received", logger.Int("body_bytes", len(respBody)))

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

	// 转换消息格式 (same logic as Chat)
	apiMsgs := make([]openaiMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
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
				apiMsgs = append(apiMsgs, openaiMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		} else {
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

	// 构建工具
	var apiTools []openaiTool
	for _, tool := range req.Tools {
		schema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
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

	apiReq := &openaiRequest{
		Model:       p.cfg.Model,
		Messages:    apiMsgs,
		Tools:       apiTools,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}

	gen := p.getGenerationCopy()
	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = gen.MaxTokens
		if apiReq.MaxTokens == 0 {
			apiReq.MaxTokens = p.cfg.MaxTokens
		}
	}
	if apiReq.Temperature == 0 {
		apiReq.Temperature = gen.Temperature
		if apiReq.Temperature == 0 {
			apiReq.Temperature = p.cfg.Temperature
		}
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

	resp, err := p.streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // 64 KB max
		resp.Body.Close()
		errResp := p.parseErrorResponse(errBody, resp.StatusCode)
		resultChan := make(chan StreamChatResponse, 1)
		resultChan <- StreamChatResponse{
			Content:      errResp.Content,
			FinishReason: "error",
			Error:        fmt.Errorf("api error: %s", resp.Status),
		}
		close(resultChan)
		return resultChan, nil
	}

	resultChan := make(chan StreamChatResponse)
	go func() {
		defer close(resultChan)
		defer resp.Body.Close()

		p.streamSSE(ctx, resp.Body, resultChan)
	}()

	return resultChan, nil
}

// streamSSE parses Server-Sent Events from the response body and sends chunks.
func (p *GenericProvider) streamSSE(ctx context.Context, body io.Reader, ch chan<- StreamChatResponse) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	currentData := ""
	const maxBufSize = 1 * 1024 * 1024 // 1 MB buffer limit

	// sendSSE 安全地发送到 channel，检查 context 取消防止 goroutine 泄漏
	sendSSE := func(resp StreamChatResponse) bool {
		select {
		case ch <- resp:
			return true
		case <-ctx.Done():
			return false
		}
	}

	for {
		select {
		case <-ctx.Done():
			sendSSE(StreamChatResponse{Error: ctx.Err()})
			return
		default:
		}

		n, err := body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}

		// P0-8: 防止缓冲区无限增长
		if len(buf) > maxBufSize {
			sendSSE(StreamChatResponse{Error: fmt.Errorf("SSE buffer exceeded %d bytes, aborting", maxBufSize)})
			return
		}

		// Process complete lines
		for {
			idx := bytes.IndexByte(buf, '\n')
			if idx < 0 {
				break
			}
			line := buf[:idx]
			buf = buf[idx+1:]

			// Trim \r
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}

			lineStr := string(line)

			// P1-8: 支持 "data:" 和 "data: " 两种格式
			if strings.HasPrefix(lineStr, "data:") {
				data := strings.TrimSpace(lineStr[5:])
				if data == "[DONE]" {
					sendSSE(StreamChatResponse{FinishReason: "stop"})
					return
				}
				currentData = data
			} else if lineStr == "" && currentData != "" {
				// Empty line signals end of event — process currentData
				p.processStreamChunk(currentData, ch)
				currentData = ""
			}
		}

		if err != nil {
			if err != io.EOF {
				sendSSE(StreamChatResponse{Error: err})
			}
			// Process any remaining data
			if currentData != "" {
				p.processStreamChunk(currentData, ch)
			}
			return
		}
	}
}

// processStreamChunk parses a single SSE data chunk and sends it to the channel.
func (p *GenericProvider) processStreamChunk(data string, ch chan<- StreamChatResponse) {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Index    int `json:"index"`
					ID       string `json:"id,omitempty"`
					Type     string `json:"type,omitempty"`
					Function struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					} `json:"function,omitempty"`
				} `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logger.Debug("Failed to parse stream chunk", logger.String("data", data), logger.ErrorField(err))
		return
	}

	for _, choice := range chunk.Choices {
		resp := StreamChatResponse{
			Content: choice.Delta.Content,
		}
		if choice.FinishReason != nil {
			resp.FinishReason = *choice.FinishReason
		}
		for _, tc := range choice.Delta.ToolCalls {
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
		ch <- resp
	}
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
