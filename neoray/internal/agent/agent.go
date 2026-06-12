package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/tools"
)

// Agent AI 代理
type Agent struct {
	cfg            *config.Config
	providerMgr    *provider.ProviderManager
	sessionMgr     *session.Manager
	contextBuilder *ContextBuilder
	toolRegistry   *tools.Registry
	tokenManager   *TokenManager
	traceManager   *TraceManager
}

// AgentOption Agent 配置选项
type AgentOption func(*Agent)

// WithTokenManager 设置 Token 管理器
func WithTokenManager(tm *TokenManager) AgentOption {
	return func(a *Agent) {
		a.tokenManager = tm
	}
}

// WithTraceManager 设置追踪管理器
func WithTraceManager(tm *TraceManager) AgentOption {
	return func(a *Agent) {
		a.traceManager = tm
	}
}

// NewAgent 创建 Agent
func NewAgent(
	cfg *config.Config,
	providerMgr *provider.ProviderManager,
	sessionMgr *session.Manager,
	toolRegistry *tools.Registry,
	opts ...AgentOption,
) *Agent {
	a := &Agent{
		cfg:            cfg,
		providerMgr:    providerMgr,
		sessionMgr:     sessionMgr,
		contextBuilder: NewContextBuilder(cfg),
		toolRegistry:   toolRegistry,
	}

	for _, opt := range opts {
		opt(a)
	}

	// 默认值
	if a.tokenManager == nil {
		a.tokenManager = NewTokenManager(0) // 无限制
	}
	if a.traceManager == nil {
		a.traceManager = NewTraceManager(false) // 默认禁用
	}

	return a
}

// ChatResult 聊天结果
type ChatResult struct {
	Message     *session.Message
	TokenUsage  *TokenUsage
	Trace       *TraceSession
	ToolCalls   int
	Iterations  int
	Duration    time.Duration
}

// Chat 发送聊天消息
func (a *Agent) Chat(ctx context.Context, sess *session.Session, userInput string) (*ChatResult, error) {
	startTime := time.Now()

	// 获取或创建追踪会话
	var trace *TraceSession
	if a.traceManager.IsEnabled() {
		trace = a.traceManager.GetOrCreateSession(sess.ID)
		trace.AddInfo("开始聊天", map[string]interface{}{
			"session_id": sess.ID,
		})
	}

	// 添加用户消息
	userMsg := session.NewUserMessage(userInput)
	sess.AddMessage(userMsg)

	// 获取提供商 - 先尝试默认的，如果没有就用任意一个可用的
	p, err := a.providerMgr.GetProvider(a.cfg.LLM.DefaultProvider)
	if err != nil || p == nil {
		// 默认 provider 不可用，尝试获取任意一个可用的
		p = a.providerMgr.DefaultProvider()
	}

	if p == nil {
		// 如果还是没有可用的 provider，给用户一个友好的错误
		errMsg := "⚠️ No LLM provider configured! Please edit your config.toml and add an API key for Anthropic or OpenAI."
		logger.Error("No LLM provider available", logger.String("default_provider", a.cfg.LLM.DefaultProvider))

		if trace != nil {
			trace.AddError(errors.New(errMsg), "No LLM provider configured")
		}

		assistantMsg := session.NewAssistantMessage(errMsg)
		sess.AddMessage(assistantMsg)
		_ = a.sessionMgr.SaveSession(sess)

		return &ChatResult{
			Message:    &assistantMsg,
			TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
			Trace:      trace,
			ToolCalls:  0,
			Iterations: 1,
			Duration:   time.Since(startTime),
		}, nil
	}

	// 构建工具定义
	var providerTools []provider.Tool
	if a.toolRegistry != nil {
		for _, def := range a.toolRegistry.GetDefinitions() {
			var schema map[string]interface{}
			_ = json.Unmarshal(def.InputSchema, &schema)
			providerTools = append(providerTools, provider.Tool{
				Name:        def.Name,
				Description: def.Description,
				InputSchema: schema,
			})
		}
	}

	var totalToolCalls int
	var iterations int

	// 工具调用循环（最多 10 次）
	maxIterations := 10
	for iterations = 0; iterations < maxIterations; iterations++ {
		select {
		case <-ctx.Done():
			if trace != nil {
				trace.AddError(ctx.Err(), "上下文取消")
			}
			return nil, ctx.Err()
		default:
		}

		iterStartTime := time.Now()

		// 构建上下文
		msgs := a.contextBuilder.BuildMessages(sess)

		// 调用 LLM
		req := &provider.ChatRequest{
			Messages:    msgs,
			Tools:       providerTools,
		}

		// 从配置中获取当前 provider 的参数
		if providerCfg, ok := a.cfg.LLM.Providers[p.Name()]; ok {
			req.MaxTokens = providerCfg.MaxTokens
			req.Temperature = providerCfg.Temperature
		}

		logger.Debug("Calling LLM",
			logger.String("session_id", sess.ID),
			logger.String("provider", p.Name()),
			logger.Int("iteration", iterations+1),
		)

		resp, err := a.callLLMWithRetry(ctx, p, req)
		iterDuration := time.Since(iterStartTime)

		if err != nil {
			logger.Error("LLM call failed after retries",
				logger.ErrorField(err),
				logger.Int("iteration", iterations+1),
			)
			if trace != nil {
				trace.AddError(err, fmt.Sprintf("LLM 调用失败 (迭代 %d)", iterations+1))
			}

			// 尝试给用户返回一个错误消息
			errMsg := fmt.Sprintf("I'm having trouble connecting to the AI service right now. Error: %v", err)
			assistantMsg := session.NewAssistantMessage(errMsg)
			sess.AddMessage(assistantMsg)
			_ = a.sessionMgr.SaveSession(sess)

			return &ChatResult{
				Message:    &assistantMsg,
				TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
				Trace:      trace,
				ToolCalls:  totalToolCalls,
				Iterations: iterations + 1,
				Duration:   time.Since(startTime),
			}, nil
		}

		if trace != nil {
			trace.AddLLMCall(iterations+1, 0, 0, iterDuration)
		}

		// 添加助手消息
		assistantMsg := session.NewAssistantMessage(resp.Content)
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
		}

		sess.AddMessage(assistantMsg)

		// 如果没有工具调用，直接返回
		if len(resp.ToolCalls) == 0 || a.toolRegistry == nil {
			// 保存会话
			if err := a.sessionMgr.SaveSession(sess); err != nil {
				logger.Warn("Failed to save session", logger.ErrorField(err))
			}

			return &ChatResult{
				Message:    &assistantMsg,
				TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
				Trace:      trace,
				ToolCalls:  totalToolCalls,
				Iterations: iterations + 1,
				Duration:   time.Since(startTime),
			}, nil
		}

		// 执行工具调用
		totalToolCalls += len(resp.ToolCalls)
		toolResponses := a.executeToolCalls(ctx, resp.ToolCalls, trace)

		// 添加工具响应消息
		toolRespJSON, _ := json.Marshal(toolResponses)
		toolMsg := session.NewToolMessage(string(toolRespJSON))
		sess.AddMessage(toolMsg)
	}

	// 超过最大迭代次数，返回最后的消息
	logger.Warn("Max tool iterations reached")
	if trace != nil {
		trace.AddInfo("达到最大迭代次数", map[string]interface{}{
			"max_iterations": maxIterations,
		})
	}

	if len(sess.Messages) > 0 {
		lastMsg := sess.Messages[len(sess.Messages)-1]
		_ = a.sessionMgr.SaveSession(sess)
		return &ChatResult{
			Message:    &lastMsg,
			TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
			Trace:      trace,
			ToolCalls:  totalToolCalls,
			Iterations: iterations,
			Duration:   time.Since(startTime),
		}, nil
	}

	return nil, errors.New("no response generated")
}

// callLLMWithRetry 带重试的 LLM 调用
func (a *Agent) callLLMWithRetry(ctx context.Context, p provider.Provider, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		logger.Warn("LLM call failed, retrying",
			logger.ErrorField(err),
			logger.Int("attempt", attempt+1),
		)

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<attempt) // 指数退避
			logger.Debug("Waiting before retry",
				logger.Duration("delay", delay),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}

// executeToolCalls 执行工具调用
func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall, trace *TraceSession) []map[string]interface{} {
	var toolResponses []map[string]interface{}

	// 并行执行工具调用
	var wg sync.WaitGroup
	var mu sync.Mutex
	toolResponses = make([]map[string]interface{}, len(toolCalls))

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc provider.ToolCall) {
			defer wg.Done()

			toolStartTime := time.Now()

			logger.Debug("Executing tool",
				logger.String("tool", tc.Name),
				logger.String("id", tc.ID),
			)

			// 添加超时
			toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			result, err := a.toolRegistry.Execute(toolCtx, tc.Name, json.RawMessage(tc.Arguments))
			toolDuration := time.Since(toolStartTime)

			if err != nil {
				logger.Warn("Tool execution failed",
					logger.String("tool", tc.Name),
					logger.ErrorField(err),
				)
				if trace != nil {
					trace.AddToolCall(tc.Name, tc.ID, true, toolDuration)
				}
				mu.Lock()
				toolResponses[idx] = map[string]interface{}{
					"tool_use_id": tc.ID,
					"content":     fmt.Sprintf("Error: %v", err),
					"is_error":    true,
				}
				mu.Unlock()
			} else {
				if trace != nil {
					trace.AddToolCall(tc.Name, tc.ID, false, toolDuration)
				}
				mu.Lock()
				toolResponses[idx] = map[string]interface{}{
					"tool_use_id": tc.ID,
					"content":     string(result),
				}
				mu.Unlock()
			}
		}(i, tc)
	}

	wg.Wait()
	return toolResponses
}

// StreamChunk 流式响应块
type StreamChunk struct {
	Type         string                 // "text", "tool_start", "tool_result", "end", "error"
	Content      string                 // 文本内容
	ToolCalls    []provider.ToolCall    // 工具调用
	ToolResults  []map[string]interface{} // 工具结果
	Error        error                  // 错误
	SessionMsg   *session.Message       // 完整的会话消息（仅在 end 时）
}

// ChatStream 流式聊天（支持工具调用）
func (a *Agent) ChatStream(ctx context.Context, sess *session.Session, userInput string) (<-chan StreamChunk, error) {
	resultChan := make(chan StreamChunk, 100)

	// 添加用户消息
	userMsg := session.NewUserMessage(userInput)
	sess.AddMessage(userMsg)

	go func() {
		defer close(resultChan)

		// 获取或创建追踪会话
		var trace *TraceSession
		if a.traceManager.IsEnabled() {
			trace = a.traceManager.GetOrCreateSession(sess.ID)
		}

		// 获取提供商 - 先尝试默认的，如果没有就用任意一个可用的
		p, err := a.providerMgr.GetProvider(a.cfg.LLM.DefaultProvider)
		if err != nil || p == nil {
			p = a.providerMgr.DefaultProvider()
		}

		if p == nil {
			resultChan <- StreamChunk{Type: "error", Error: fmt.Errorf("no LLM provider configured")}
			return
		}

		// 构建工具定义
		var providerTools []provider.Tool
		if a.toolRegistry != nil {
			for _, def := range a.toolRegistry.GetDefinitions() {
				var schema map[string]interface{}
				_ = json.Unmarshal(def.InputSchema, &schema)
				providerTools = append(providerTools, provider.Tool{
					Name:        def.Name,
					Description: def.Description,
					InputSchema: schema,
				})
			}
		}

		// 工具调用循环
		maxIterations := 10
		for iteration := 0; iteration < maxIterations; iteration++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// 构建上下文
			msgs := a.contextBuilder.BuildMessages(sess)

			// 先尝试流式获取响应
			req := &provider.ChatRequest{
				Messages:    msgs,
				Tools:       providerTools,
				MaxTokens:   0,
				Temperature: 0,
				Stream:      true,
			}

			// 检查提供商是否支持原生流式工具调用
			if streamProvider, ok := p.(provider.StreamToolProvider); ok {
				// 使用原生流式工具调用
				done, err := a.handleNativeStreamTool(ctx, streamProvider, req, sess, resultChan, trace)
				if err != nil {
					resultChan <- StreamChunk{Type: "error", Error: err}
					return
				}
				if done {
					return
				}
			} else {
				// 回退到非流式处理，但可以流式输出文本
				done, err := a.handleFallbackStream(ctx, p, req, sess, resultChan, trace)
				if err != nil {
					resultChan <- StreamChunk{Type: "error", Error: err}
					return
				}
				if done {
					return
				}
			}
		}

		// 超过最大迭代
		logger.Warn("Max tool iterations reached in stream")
	}()

	return resultChan, nil
}

// handleNativeStreamTool 处理原生流式工具调用
func (a *Agent) handleNativeStreamTool(
	ctx context.Context,
	p provider.StreamToolProvider,
	req *provider.ChatRequest,
	sess *session.Session,
	resultChan chan<- StreamChunk,
	trace *TraceSession,
) (bool, error) {

	stream, err := p.ChatStreamWithTools(ctx, req)
	if err != nil {
		return false, err
	}

	var fullContent string
	var currentToolCalls []provider.ToolCall
	var toolCallInProgress bool

	for chunk := range stream {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		default:
		}

		if chunk.Error != nil {
			logger.Error("Stream error", logger.ErrorField(chunk.Error))
			return false, chunk.Error
		}

		// 处理文本内容
		if chunk.Content != "" {
			fullContent += chunk.Content
			resultChan <- StreamChunk{
				Type:    "text",
				Content: chunk.Content,
			}
		}

		// 处理工具调用开始
		if len(chunk.ToolCalls) > 0 && !toolCallInProgress {
			toolCallInProgress = true
			currentToolCalls = append(currentToolCalls, chunk.ToolCalls...)
			resultChan <- StreamChunk{
				Type:      "tool_start",
				ToolCalls: chunk.ToolCalls,
			}
		}

		// 处理完成
		if chunk.FinishReason != "" {
			logger.Debug("Stream finished", logger.String("reason", chunk.FinishReason))

			// 有工具调用，需要执行
			if len(currentToolCalls) > 0 {
				// 保存助手消息
				assistantMsg := session.NewAssistantMessage(fullContent)
				if len(currentToolCalls) > 0 {
					assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(currentToolCalls))
					for _, tc := range currentToolCalls {
						assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{
							ID:        tc.ID,
							Name:      tc.Name,
							Arguments: tc.Arguments,
						})
					}
				}
				sess.AddMessage(assistantMsg)

				// 执行工具调用
				toolResponses := a.executeToolCalls(ctx, currentToolCalls, trace)

				// 发送工具结果
				resultChan <- StreamChunk{
					Type:        "tool_result",
					ToolResults: toolResponses,
				}

				// 添加工具响应消息
				toolRespJSON, _ := json.Marshal(toolResponses)
				toolMsg := session.NewToolMessage(string(toolRespJSON))
				sess.AddMessage(toolMsg)

				// 继续循环以获取下一个 LLM 响应
				return false, nil
			}

			// 没有工具调用，保存并完成
			if fullContent != "" {
				assistantMsg := session.NewAssistantMessage(fullContent)
				sess.AddMessage(assistantMsg)
				if err := a.sessionMgr.SaveSession(sess); err != nil {
					logger.Warn("Failed to save session", logger.ErrorField(err))
				}

				resultChan <- StreamChunk{
					Type:       "end",
					Content:    fullContent,
					SessionMsg: &assistantMsg,
				}
			}

			return true, nil
		}
	}

	return true, nil
}

// handleFallbackStream 回退流式处理（非流式 LLM 调用但流式输出）
func (a *Agent) handleFallbackStream(
	ctx context.Context,
	p provider.Provider,
	req *provider.ChatRequest,
	sess *session.Session,
	resultChan chan<- StreamChunk,
	trace *TraceSession,
) (bool, error) {

	// 使用非流式调用
	resp, err := a.callLLMWithRetry(ctx, p, req)
	if err != nil {
		return false, err
	}

	// 流式输出文本（模拟）
	if resp.Content != "" {
		// 简单地按字符发送
		for _, c := range resp.Content {
			select {
			case <-ctx.Done():
				return true, ctx.Err()
			case <-time.After(5 * time.Millisecond): // 模拟延迟
				resultChan <- StreamChunk{
					Type:    "text",
					Content: string(c),
				}
			}
		}
	}

	// 有工具调用
	if len(resp.ToolCalls) > 0 && a.toolRegistry != nil {
		// 发送工具开始
		resultChan <- StreamChunk{
			Type:      "tool_start",
			ToolCalls: resp.ToolCalls,
		}

		// 保存助手消息
		assistantMsg := session.NewAssistantMessage(resp.Content)
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
		}
		sess.AddMessage(assistantMsg)

		// 执行工具调用
		toolResponses := a.executeToolCalls(ctx, resp.ToolCalls, trace)

		// 发送工具结果
		resultChan <- StreamChunk{
			Type:        "tool_result",
			ToolResults: toolResponses,
		}

		// 添加工具响应消息
		toolRespJSON, _ := json.Marshal(toolResponses)
		toolMsg := session.NewToolMessage(string(toolRespJSON))
		sess.AddMessage(toolMsg)

		// 继续循环
		return false, nil
	}

	// 没有工具调用，完成
	assistantMsg := session.NewAssistantMessage(resp.Content)
	sess.AddMessage(assistantMsg)
	if err := a.sessionMgr.SaveSession(sess); err != nil {
		logger.Warn("Failed to save session", logger.ErrorField(err))
	}

	resultChan <- StreamChunk{
		Type:       "end",
		Content:    resp.Content,
		SessionMsg: &assistantMsg,
	}

	return true, nil
}
