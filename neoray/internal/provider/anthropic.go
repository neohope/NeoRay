package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// AnthropicProvider Anthropic API 提供商
type AnthropicProvider struct {
	name         string
	cfg          *config.ProviderConfig
	client       *http.Client       // 带超时，用于非流式请求
	streamClient *http.Client       // 无超时，用于 SSE 流式请求（依赖 context 取消）
	generation   GenerationSettings
	extraHeaders map[string]string
}

// NewAnthropicProvider 创建 Anthropic 提供商
func NewAnthropicProvider(name string, cfg *config.ProviderConfig) *AnthropicProvider {
	return &AnthropicProvider{
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
		extraHeaders: make(map[string]string),
	}
}

// Name 提供商名称
func (p *AnthropicProvider) Name() string {
	return p.name
}

// GetGenerationSettings 获取生成设置
func (p *AnthropicProvider) GetGenerationSettings() GenerationSettings {
	return p.generation
}

// SetGenerationSettings 设置生成设置
func (p *AnthropicProvider) SetGenerationSettings(settings GenerationSettings) {
	p.generation = settings
}

// GetDefaultModel 获取默认模型
func (p *AnthropicProvider) GetDefaultModel() string {
	return p.cfg.Model
}

// SetExtraHeader 设置额外的请求头
func (p *AnthropicProvider) SetExtraHeader(key, value string) {
	p.extraHeaders[key] = value
}

// anthropicRequest Anthropic API 请求
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      interface{}        `json:"system,omitempty"` // string or []anthropicSystemBlock
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Thinking    *anthropicThinking `json:"thinking,omitempty"`
	ToolChoice  interface{}        `json:"tool_choice,omitempty"`
}

// anthropicSystemBlock 系统消息块（支持缓存）
type anthropicSystemBlock struct {
	Type         string                 `json:"type"` // "text"
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

// anthropicCacheControl 缓存控制
type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// anthropicThinking 思考模式配置
type anthropicThinking struct {
	Type         string `json:"type"` // "enabled" 或 "adaptive"
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// anthropicMessage Anthropic 消息
type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

// anthropicContentBlock 内容块
type anthropicContentBlock struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result", "thinking"

	// Text block
	Text         string                 `json:"text,omitempty"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`

	// Tool use block
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`

	// Tool result block
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`

	// Thinking block
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// anthropicTool Anthropic 工具定义
type anthropicTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

// anthropicResponse Anthropic API 响应
type anthropicResponse struct {
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage"`
}

// anthropicStreamEvent 流式事件
type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
	Message *anthropicResponse `json:"message,omitempty"`
}

// anthropicTextDelta 文本 delta
type anthropicTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicThinkingDelta 思考 delta
type anthropicThinkingDelta struct {
	Type     string `json:"type"`
	Thinking string `json:"thinking"`
}

// anthropicToolUseDelta 工具使用 delta
type anthropicToolUseDelta struct {
	Type        string          `json:"type"`
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	PartialJSON json.RawMessage `json:"partial_json,omitempty"`
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
		logger.String("reasoning_effort", req.ReasoningEffort),
		logger.Bool("cache_enabled", req.CacheEnabled),
	)

	// 构建请求
	apiReq, err := p.buildRequest(req)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	logger.Debug("Anthropic API request", logger.String("body", string(body)))

	// 发送请求
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
	// 构建 beta 头（多个 beta 特性用逗号分隔）
	var betaFeatures []string
	if req.CacheEnabled {
		betaFeatures = append(betaFeatures, "prompt-caching-2024-07-31")
	}
	if req.ReasoningEffort != "" && req.ReasoningEffort != "none" {
		betaFeatures = append(betaFeatures, "extended-thinking-2025-07-31")
	}
	if len(betaFeatures) > 0 {
		httpReq.Header.Set("anthropic-beta", strings.Join(betaFeatures, ","))
	}
	// 添加额外的请求头
	for k, v := range p.extraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return p.handleError(err, nil)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		errResp := p.parseErrorResponse(errBody)
		errResp.ErrorStatusCode = resp.StatusCode
		return errResp, fmt.Errorf("api error: %s", resp.Status)
	}

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	logger.Debug("Anthropic API raw response", logger.String("body", string(respBody)))

	// 解析响应
	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 转换为通用响应
	return p.parseResponse(&apiResp), nil
}

// ChatStream 流式聊天
func (p *AnthropicProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error) {
	if p.cfg.APIKey == "" {
		return nil, fmt.Errorf("%s api key not configured", p.name)
	}

	logger.Debug("Calling Anthropic API (stream)",
		logger.String("provider", p.name),
		logger.String("model", p.cfg.Model),
		logger.Int("tools_count", len(req.Tools)),
	)

	resultChan := make(chan StreamChatResponse)

	// 构建请求
	apiReq, err := p.buildRequest(req)
	if err != nil {
		close(resultChan)
		return nil, err
	}
	apiReq.Stream = true

	body, err := json.Marshal(apiReq)
	if err != nil {
		close(resultChan)
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 发送请求
	apiURL := p.cfg.APIURL
	if apiURL == "" {
		apiURL = "https://api.anthropic.com"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		close(resultChan)
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	// 构建 beta 头（多个 beta 特性用逗号分隔）
	var betaFeatures []string
	if req.CacheEnabled {
		betaFeatures = append(betaFeatures, "prompt-caching-2024-07-31")
	}
	if req.ReasoningEffort != "" && req.ReasoningEffort != "none" {
		betaFeatures = append(betaFeatures, "extended-thinking-2025-07-31")
	}
	if len(betaFeatures) > 0 {
		httpReq.Header.Set("anthropic-beta", strings.Join(betaFeatures, ","))
	}
	// 添加额外的请求头
	for k, v := range p.extraHeaders {
		httpReq.Header.Set(k, v)
	}

	// 在 goroutine 中处理流式响应
	go func() {
		defer close(resultChan)

		resp, err := p.streamClient.Do(httpReq)
		if err != nil {
			_, _ = p.handleError(err, nil)
			resultChan <- StreamChatResponse{Error: fmt.Errorf("do request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(resp.Body)
			_ = p.parseErrorResponse(errBody)
			resultChan <- StreamChatResponse{
				Error: fmt.Errorf("api error: %s, body: %s", resp.Status, string(errBody)),
			}
			return
		}

		p.handleStreamResponse(ctx, resp.Body, resultChan)
	}()

	return resultChan, nil
}

// buildRequest 构建 API 请求
func (p *AnthropicProvider) buildRequest(req *ChatRequest) (*anthropicRequest, error) {
	// 分离系统消息和普通消息
	var systemMessage string
	var systemBlocks []anthropicSystemBlock
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
			// 添加思考块
			for _, tb := range msg.ThinkingBlocks {
				contentBlocks = append(contentBlocks, anthropicContentBlock{
					Type:      tb.Type,
					Thinking:  tb.Thinking,
					Signature: tb.Signature,
				})
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

	// 合并相同角色的连续消息
	apiMsgs = mergeConsecutiveMessages(apiMsgs)

	// 构建工具定义
	var apiTools []anthropicTool
	for _, tool := range req.Tools {
		logger.Debug("Adding tool to request",
			logger.String("name", tool.Name),
			logger.String("description", tool.Description),
		)
		schema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			logger.Warn("Skipping tool with invalid InputSchema", logger.String("tool", tool.Name))
			continue
		}
		apiTool := anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		}
		if tool.CacheControl != nil && req.CacheEnabled {
			apiTool.CacheControl = &anthropicCacheControl{
				Type: tool.CacheControl.Type,
			}
		}
		apiTools = append(apiTools, apiTool)
	}

	// 构建请求
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.generation.MaxTokens
		if maxTokens == 0 {
			maxTokens = p.cfg.MaxTokens
		}
	}

	temperature := req.Temperature
	if temperature == 0 {
		temperature = p.generation.Temperature
		if temperature == 0 {
			temperature = p.cfg.Temperature
		}
	}

	reasoningEffort := req.ReasoningEffort
	if reasoningEffort == "" {
		reasoningEffort = p.generation.ReasoningEffort
	}

	apiReq := &anthropicRequest{
		Model:     p.cfg.Model,
		Messages:  apiMsgs,
		MaxTokens: maxTokens,
		Tools:     apiTools,
	}

	// 处理系统消息（支持缓存）
	if systemMessage != "" {
		if req.CacheEnabled {
			systemBlocks = []anthropicSystemBlock{{
				Type: "text",
				Text: systemMessage,
				CacheControl: &anthropicCacheControl{
					Type: "ephemeral",
				},
			}}
			apiReq.System = systemBlocks
		} else {
			apiReq.System = systemMessage
		}
	}

	// 处理思考模式
	if reasoningEffort != "" && reasoningEffort != "none" {
		if reasoningEffort == "adaptive" {
			apiReq.Thinking = &anthropicThinking{
				Type: "adaptive",
			}
			// 自适应思考模式强制设置 temperature 为 1
			apiReq.Temperature = 1.0
		} else {
			// 预算映射
			budgetMap := map[string]int{
				"low":    1024,
				"medium": 4096,
				"high":   max(maxTokens, 8192),
			}
			budget := budgetMap[reasoningEffort]
			if budget == 0 {
				budget = 4096 // 默认
			}
			apiReq.Thinking = &anthropicThinking{
				Type:         "enabled",
				BudgetTokens: budget,
			}
			// 确保 max_tokens 足够大
			apiReq.MaxTokens = max(maxTokens, budget+4096)
			apiReq.Temperature = 1.0
		}
	} else {
		// Opus 4.7 不支持 temperature 参数
		if !strings.Contains(p.cfg.Model, "opus-4-7") {
			apiReq.Temperature = temperature
		}
	}

	// 应用缓存控制到消息
	if req.CacheEnabled && len(apiMsgs) >= 3 {
		// 给倒数第二条消息的最后一个文本块添加缓存标记
		secondLastIdx := len(apiMsgs) - 2
		if secondLastIdx >= 0 {
			msg := &apiMsgs[secondLastIdx]
			for i := len(msg.Content) - 1; i >= 0; i-- {
				block := &msg.Content[i]
				if block.Type == "text" {
					block.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
					break
				}
			}
		}

		// 给最后一个工具添加缓存标记
		if len(apiTools) > 0 {
			lastTool := &apiTools[len(apiTools)-1]
			lastTool.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
		}
	}

	return apiReq, nil
}

// parseResponse 解析响应
func (p *AnthropicProvider) parseResponse(apiResp *anthropicResponse) *ChatResponse {
	var content string
	var toolCalls []ToolCall
	var thinkingBlocks []ThinkingBlock

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
		case "thinking":
			thinkingBlocks = append(thinkingBlocks, ThinkingBlock{
				Type:      block.Type,
				Thinking:  block.Thinking,
				Signature: block.Signature,
			})
		}
	}

	// 转换停止原因
	stopMap := map[string]string{
		"tool_use":   "tool_calls",
		"end_turn":   "stop",
		"max_tokens": "length",
	}
	finishReason := stopMap[apiResp.StopReason]
	if finishReason == "" {
		finishReason = apiResp.StopReason
	}

	// 构建使用量
	usage := &Usage{
		InputTokens:              apiResp.Usage.InputTokens,
		OutputTokens:             apiResp.Usage.OutputTokens,
		CacheCreationInputTokens: apiResp.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     apiResp.Usage.CacheReadInputTokens,
	}
	if apiResp.Usage.CacheReadInputTokens > 0 {
		usage.CachedTokens = apiResp.Usage.CacheReadInputTokens
	}

	response := &ChatResponse{
		Content:        content,
		ToolCalls:      toolCalls,
		ThinkingBlocks: thinkingBlocks,
		FinishReason:   finishReason,
		Usage:          usage,
	}

	logger.Debug("Anthropic API response",
		logger.Int("input_tokens", apiResp.Usage.InputTokens),
		logger.Int("output_tokens", apiResp.Usage.OutputTokens),
		logger.Int("cache_creation_tokens", apiResp.Usage.CacheCreationInputTokens),
		logger.Int("cache_read_tokens", apiResp.Usage.CacheReadInputTokens),
		logger.Int("tool_calls", len(response.ToolCalls)),
		logger.Int("thinking_blocks", len(thinkingBlocks)),
	)

	return response
}

// handleStreamResponse 处理流式响应
func (p *AnthropicProvider) handleStreamResponse(ctx context.Context, body io.Reader, resultChan chan<- StreamChatResponse) {
	scanner := bufio.NewScanner(body)
	toolBlocks := make(map[int]*ToolCall)
	var currentThinking string

	for {
		select {
		case <-ctx.Done():
			resultChan <- StreamChatResponse{Error: ctx.Err()}
			return
		default:
		}

		var line []byte
		var event anthropicStreamEvent

		// 读取 SSE 格式
		for scanner.Scan() {
			rawLine := scanner.Bytes()
			lineStr := string(rawLine)
			lineStr = strings.TrimSpace(lineStr)

			if lineStr == "" {
				// 空行分隔事件
				if len(line) > 0 {
					// 尝试解析 data: 开头的行
					if bytes.HasPrefix(line, []byte("data: ")) {
						data := bytes.TrimPrefix(line, []byte("data: "))
						if err := json.Unmarshal(data, &event); err == nil {
							break
						}
					}
					line = nil
				}
				if scanner.Err() == io.EOF {
					return
				}
				continue
			}

			if strings.HasPrefix(lineStr, "data: ") {
				line = append(line, rawLine...)
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			resultChan <- StreamChatResponse{Error: err}
			return
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				logger.Debug("Stream message start",
					logger.Int("input_tokens", event.Message.Usage.InputTokens),
					logger.Int("cache_creation_tokens", event.Message.Usage.CacheCreationInputTokens),
					logger.Int("cache_read_tokens", event.Message.Usage.CacheReadInputTokens),
				)
			}

		case "content_block_start":
			if len(event.Delta) > 0 {
				var delta anthropicContentBlock
				if err := json.Unmarshal(event.Delta, &delta); err == nil {
					switch delta.Type {
					case "tool_use":
						toolBlocks[event.Index] = &ToolCall{
							ID:   delta.ID,
							Name: delta.Name,
						}
						resultChan <- StreamChatResponse{
							ToolCalls: []ToolCall{*toolBlocks[event.Index]},
						}
					case "thinking":
						// 思考块开始
					}
				}
			}

		case "content_block_delta":
			if len(event.Delta) > 0 {
				// 尝试文本 delta
				var textDelta anthropicTextDelta
				if err := json.Unmarshal(event.Delta, &textDelta); err == nil {
					if textDelta.Text != "" {
						resultChan <- StreamChatResponse{
							Content: textDelta.Text,
						}
					}
					continue
				}

				// 尝试思考 delta
				var thinkingDelta anthropicThinkingDelta
				if err := json.Unmarshal(event.Delta, &thinkingDelta); err == nil {
					if thinkingDelta.Thinking != "" {
						currentThinking += thinkingDelta.Thinking
						resultChan <- StreamChatResponse{
							ThinkingDelta: thinkingDelta.Thinking,
						}
					}
					continue
				}

				// 尝试工具使用 delta
				var toolDelta anthropicToolUseDelta
				if err := json.Unmarshal(event.Delta, &toolDelta); err == nil {
					if tb, ok := toolBlocks[event.Index]; ok {
						tb.Arguments += string(toolDelta.PartialJSON)
						resultChan <- StreamChatResponse{
							ToolCalls: []ToolCall{*tb},
						}
					}
					continue
				}
			}

		case "message_delta":
			if event.Usage != nil {
				logger.Debug("Stream message delta",
					logger.Int("output_tokens", event.Usage.OutputTokens),
				)
			}

		case "message_stop":
			resultChan <- StreamChatResponse{
				FinishReason: "stop",
			}
			return

		case "error":
			var errResp struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(event.Delta, &errResp) == nil {
				resultChan <- StreamChatResponse{
					Error: fmt.Errorf("%s: %s", errResp.Error.Type, errResp.Error.Message),
				}
			} else {
				resultChan <- StreamChatResponse{
					Error: fmt.Errorf("stream error: %s", string(event.Delta)),
				}
			}
			return
		}
	}
}

// handleError 处理错误
func (p *AnthropicProvider) handleError(err error, headers http.Header) (*ChatResponse, error) {
	errResp := &ChatResponse{
		Content:      fmt.Sprintf("Error calling LLM: %v", err),
		FinishReason: "error",
	}

	// 提取重试信息
	if headers != nil {
		if retryAfter := p.extractRetryAfterFromHeaders(headers); retryAfter > 0 {
			errResp.RetryAfter = retryAfter
		}
	}

	// 尝试从错误信息中提取重试信息
	errMsg := err.Error()
	if retryAfter := p.extractRetryAfter(errMsg); retryAfter > 0 {
		errResp.RetryAfter = retryAfter
	}

	// 检查错误类型
	if strings.Contains(errMsg, "timeout") {
		errResp.ErrorType = "timeout"
		errResp.ErrorShouldRetry = true
	} else if strings.Contains(errMsg, "connection") {
		errResp.ErrorType = "connection"
		errResp.ErrorShouldRetry = true
	}

	return errResp, err
}

// parseErrorResponse 解析错误响应
func (p *AnthropicProvider) parseErrorResponse(body []byte) *ChatResponse {
	errResp := &ChatResponse{
		FinishReason: "error",
	}

	var errStruct struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errStruct); err == nil {
		errResp.Content = errStruct.Error.Message
		errResp.ErrorType = errStruct.Error.Type
		errResp.ErrorCode = errStruct.Error.Type

		// 判断是否应该重试
		if strings.Contains(errStruct.Error.Type, "rate_limit") ||
			strings.Contains(errStruct.Error.Type, "overloaded") ||
			strings.Contains(errStruct.Error.Message, "retry after") {
			errResp.ErrorShouldRetry = true
		}

		// 检查是否是欠费/配额错误
		arrearageTokens := []string{
			"insufficient_quota", "quota_exceeded", "quota_exhausted",
			"billing_hard_limit", "insufficient_balance", "payment_required",
		}
		for _, token := range arrearageTokens {
			if strings.Contains(errStruct.Error.Type, token) ||
				strings.Contains(errStruct.Error.Message, token) {
				errResp.ErrorShouldRetry = false
			}
		}
	} else {
		errResp.Content = string(body)
	}

	return errResp
}

var retryAfterPatterns = regexp.MustCompile(
	`(?i)(?:retry after|try again in|wait)\s+(\d+(?:\.\d+)?)\s*(ms|milliseconds|s|sec|seconds|m|min|minutes)?`)

// extractRetryAfter 从文本中提取重试时间
func (p *AnthropicProvider) extractRetryAfter(text string) time.Duration {
	matches := retryAfterPatterns.FindStringSubmatch(text)
	if matches == nil {
		return 0
	}
	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || val <= 0 {
		return 0
	}
	unit := "s" // 默认秒
	if len(matches) > 2 {
		unit = strings.ToLower(matches[2])
	}
	switch unit {
	case "ms", "milliseconds":
		return time.Duration(val * float64(time.Millisecond))
	case "m", "min", "minutes":
		return time.Duration(val * float64(time.Minute))
	default: // s, sec, seconds, or unspecified
		return time.Duration(val * float64(time.Second))
	}
}

// extractRetryAfterFromHeaders 从响应头提取重试时间
func (p *AnthropicProvider) extractRetryAfterFromHeaders(headers http.Header) time.Duration {
	// retry-after-ms 是毫秒精度
	if retryAfterMs := headers.Get("retry-after-ms"); retryAfterMs != "" {
		if ms, err := strconv.ParseFloat(retryAfterMs, 64); err == nil && ms > 0 {
			return time.Duration(ms * float64(time.Millisecond))
		}
	}
	// retry-after 可以是秒数或 HTTP 日期
	if retryAfter := headers.Get("retry-after"); retryAfter != "" {
		if secs, err := strconv.ParseFloat(retryAfter, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
	}
	return 0
}

// mergeConsecutiveMessages 合并相同角色的连续消息
func mergeConsecutiveMessages(messages []anthropicMessage) []anthropicMessage {
	if len(messages) <= 1 {
		return messages
	}

	var result []anthropicMessage
	for _, msg := range messages {
		if len(result) > 0 && result[len(result)-1].Role == msg.Role {
			// 合并内容
			last := &result[len(result)-1]
			last.Content = append(last.Content, msg.Content...)
		} else {
			result = append(result, msg)
		}
	}

	// 移除末尾的 assistant 消息（Anthropic 不支持 prefill）
	for len(result) > 0 && result[len(result)-1].Role == "assistant" {
		// 检查是否有 tool_use，如果有就不能移除
		hasToolUse := false
		for _, block := range result[len(result)-1].Content {
			if block.Type == "tool_use" {
				hasToolUse = true
				break
			}
		}
		if hasToolUse {
			break
		}
		// 如果只有一条消息是 assistant，把它转为 user
		if len(result) == 1 {
			result[0].Role = "user"
		} else {
			result = result[:len(result)-1]
		}
	}

	// 确保第一条消息不是 assistant（除非有 tool_use）
	if len(result) > 0 && result[0].Role == "assistant" {
		hasToolUse := false
		for _, block := range result[0].Content {
			if block.Type == "tool_use" {
				hasToolUse = true
				break
			}
		}
		if !hasToolUse {
			// 插入一条 synthetic user 消息
			result = append([]anthropicMessage{{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type: "text",
					Text: "(conversation continued)",
				}},
			}}, result...)
		}
	}

	return result
}
