package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"neoray/internal/bus"
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
	msgBus         *bus.MessageBus
	hook           AgentHook
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

// WithMessageBus 设置消息总线
func WithMessageBus(mb *bus.MessageBus) AgentOption {
	return func(a *Agent) {
		a.msgBus = mb
	}
}

// WithHook 设置 Agent Hook
func WithHook(hook AgentHook) AgentOption {
	return func(a *Agent) {
		a.hook = hook
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
	if a.tokenManager == nil { a.tokenManager = NewTokenManager(0) } // 无限制
	if a.traceManager == nil { a.traceManager = NewTraceManager(false) } // 默认禁用
	if a.hook == nil { a.hook = NewBaseHook("noop") } // 默认空 Hook

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
	Error       error
}

// finishChat 完成聊天，调用 AfterIter Hook
func (a *Agent) finishChat(ctx context.Context, sess *session.Session, result *ChatResult) *ChatResult {
	if err := a.hook.AfterIter(ctx, sess, result); err != nil {
		logger.Warn("Hook AfterIter failed", logger.ErrorField(err))
	}
	return result
}

// Chat 发送聊天消息
func (a *Agent) Chat(ctx context.Context, sess *session.Session, userInput string) (*ChatResult, error) {
	startTime := time.Now()

	var trace *TraceSession
	if a.traceManager.IsEnabled() {
		trace = a.traceManager.GetOrCreateSession(sess.ID)
		trace.AddInfo("开始聊天", map[string]interface{}{"session_id": sess.ID})
	}

	if err := a.hook.BeforeIter(ctx, sess); err != nil {
		logger.Warn("Hook BeforeIter failed", logger.ErrorField(err))
	}

	userMsg := session.NewUserMessage("", "", "", userInput)
	sess.AddMessage(userMsg)

	p, err := a.providerMgr.GetProvider(a.cfg.LLM.DefaultProvider)
	if err != nil || p == nil { p = a.providerMgr.DefaultProvider() }

	if p == nil {
		errMsg := "⚠️ No LLM provider configured! Please edit your config.toml and add an API key for Anthropic or OpenAI."
		logger.Error("No LLM provider available", logger.String("default_provider", a.cfg.LLM.DefaultProvider))
		if trace != nil { trace.AddError(errors.New(errMsg), "No LLM provider configured") }
		assistantMsg := session.NewAssistantMessage("", "", "", errMsg)
		sess.AddMessage(assistantMsg)
		_ = a.sessionMgr.SaveSession(sess)
		result := &ChatResult{
			Message: &assistantMsg,
			TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
			Trace: trace,
			ToolCalls: 0,
			Iterations: 1,
			Duration: time.Since(startTime),
			Error: errors.New(errMsg),
		}
		return a.finishChat(ctx, sess, result), nil
	}

	var providerTools []provider.Tool
	if a.toolRegistry != nil {
		for _, def := range a.toolRegistry.GetDefinitions() {
			var schema map[string]interface{}
			_ = json.Unmarshal(def.InputSchema, &schema)
			providerTools = append(providerTools, provider.Tool{Name: def.Name, Description: def.Description, InputSchema: schema})
		}
	}

	var totalToolCalls int
	var iterations int
	maxIterations := 10

	for iterations = 0; iterations < maxIterations; iterations++ {
		select {
		case <-ctx.Done():
			if trace != nil { trace.AddError(ctx.Err(), "上下文取消") }
			return nil, ctx.Err()
		default:
		}

		iterStartTime := time.Now()
		msgs := a.contextBuilder.BuildMessages(sess)
		req := &provider.ChatRequest{Messages: msgs, Tools: providerTools}
		if providerCfg, ok := a.cfg.LLM.Providers[p.Name()]; ok {
			req.MaxTokens = providerCfg.MaxTokens
			req.Temperature = providerCfg.Temperature
		}

		logger.Debug("Calling LLM", logger.String("session_id", sess.ID), logger.String("provider", p.Name()), logger.Int("iteration", iterations+1))
		resp, err := a.callLLMWithRetry(ctx, p, req)
		iterDuration := time.Since(iterStartTime)

		if err != nil {
			logger.Error("LLM call failed after retries", logger.ErrorField(err), logger.Int("iteration", iterations+1))
			if trace != nil { trace.AddError(err, fmt.Sprintf("LLM 调用失败 (迭代 %d)", iterations+1)) }
			errMsg := fmt.Sprintf("I'm having trouble connecting to the AI service right now. Error: %v", err)
			assistantMsg := session.NewAssistantMessage("", "", "", errMsg)
			sess.AddMessage(assistantMsg)
			_ = a.sessionMgr.SaveSession(sess)
			result := &ChatResult{
				Message: &assistantMsg,
				TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
				Trace: trace,
				ToolCalls: totalToolCalls,
				Iterations: iterations + 1,
				Duration: time.Since(startTime),
				Error: err,
			}
			return a.finishChat(ctx, sess, result), nil
		}

		if trace != nil { trace.AddLLMCall(iterations+1, 0, 0, iterDuration) }

		assistantMsg := session.NewAssistantMessage("", "", "", resp.Content)
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
			}
		}
		sess.AddMessage(assistantMsg)

		if len(resp.ToolCalls) == 0 || a.toolRegistry == nil {
			if err := a.sessionMgr.SaveSession(sess); err != nil { logger.Warn("Failed to save session", logger.ErrorField(err)) }
			result := &ChatResult{
				Message: &assistantMsg,
				TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
				Trace: trace,
				ToolCalls: totalToolCalls,
				Iterations: iterations + 1,
				Duration: time.Since(startTime),
			}
			return a.finishChat(ctx, sess, result), nil
		}

		totalToolCalls += len(resp.ToolCalls)
		toolResponses := a.executeToolCalls(ctx, resp.ToolCalls, trace)
		toolRespJSON, _ := json.Marshal(toolResponses)
		toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
		sess.AddMessage(toolMsg)
	}

	logger.Warn("Max tool iterations reached")
	if trace != nil { trace.AddInfo("达到最大迭代次数", map[string]interface{}{"max_iterations": maxIterations}) }

	if len(sess.Messages) > 0 {
		lastMsg := sess.Messages[len(sess.Messages)-1]
		_ = a.sessionMgr.SaveSession(sess)
		result := &ChatResult{
			Message: &lastMsg,
			TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
			Trace: trace,
			ToolCalls: totalToolCalls,
			Iterations: iterations,
			Duration: time.Since(startTime),
		}
		return a.finishChat(ctx, sess, result), nil
	}

	result := &ChatResult{Error: errors.New("no response generated")}
	_ = a.finishChat(ctx, sess, result)
	return nil, errors.New("no response generated")
}

func (a *Agent) callLLMWithRetry(ctx context.Context, p provider.Provider, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	maxRetries := 3
	baseDelay := 1 * time.Second
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done(): return nil, ctx.Err()
		default:
		}

		resp, err := p.Chat(ctx, req)
		if err == nil { return resp, nil }
		lastErr = err
		logger.Warn("LLM call failed, retrying", logger.ErrorField(err), logger.Int("attempt", attempt+1))

		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<attempt)
			logger.Debug("Waiting before retry", logger.Duration("delay", delay))
			select {
			case <-time.After(delay):
			case <-ctx.Done(): return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}

func (a *Agent) executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall, trace *TraceSession) []map[string]interface{} {
	var toolResponses []map[string]interface{}
	var wg sync.WaitGroup
	var mu sync.Mutex
	toolResponses = make([]map[string]interface{}, len(toolCalls))

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc provider.ToolCall) {
			defer wg.Done()
			toolStartTime := time.Now()
			logger.Debug("Executing tool", logger.String("tool", tc.Name), logger.String("id", tc.ID))
			toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			result, err := a.toolRegistry.Execute(toolCtx, tc.Name, json.RawMessage(tc.Arguments))
			toolDuration := time.Since(toolStartTime)

			if err != nil {
				logger.Warn("Tool execution failed", logger.String("tool", tc.Name), logger.ErrorField(err))
				if trace != nil { trace.AddToolCall(tc.Name, tc.ID, true, toolDuration) }
				mu.Lock()
				toolResponses[idx] = map[string]interface{}{"tool_use_id": tc.ID, "content": fmt.Sprintf("Error: %v", err), "is_error": true}
				mu.Unlock()
			} else {
				if trace != nil { trace.AddToolCall(tc.Name, tc.ID, false, toolDuration) }
				mu.Lock()
				toolResponses[idx] = map[string]interface{}{"tool_use_id": tc.ID, "content": string(result)}
				mu.Unlock()
			}
		}(i, tc)
	}

	wg.Wait()
	return toolResponses
}

type StreamChunk struct {
	Type         string
	Content      string
	ToolCalls    []provider.ToolCall
	ToolResults  []map[string]interface{}
	Error        error
	SessionMsg   *session.Message
}

func (a *Agent) ChatStream(ctx context.Context, sess *session.Session, userInput string) (<-chan StreamChunk, error) {
	resultChan := make(chan StreamChunk, 100)

	userMsg := session.NewUserMessage("", "", "", userInput)
	sess.AddMessage(userMsg)

	if err := a.hook.BeforeStream(ctx, sess); err != nil {
		logger.Warn("Hook BeforeStream failed", logger.ErrorField(err))
	}

	go func() {
		defer close(resultChan)

		var trace *TraceSession
		if a.traceManager.IsEnabled() {
			trace = a.traceManager.GetOrCreateSession(sess.ID)
		}

		p, err := a.providerMgr.GetProvider(a.cfg.LLM.DefaultProvider)
		if err != nil || p == nil { p = a.providerMgr.DefaultProvider() }

		if p == nil {
			resultChan <- StreamChunk{Type: "error", Error: fmt.Errorf("no LLM provider configured")}
			return
		}

		var providerTools []provider.Tool
		if a.toolRegistry != nil {
			for _, def := range a.toolRegistry.GetDefinitions() {
				var schema map[string]interface{}
				_ = json.Unmarshal(def.InputSchema, &schema)
				providerTools = append(providerTools, provider.Tool{Name: def.Name, Description: def.Description, InputSchema: schema})
			}
		}

		maxIterations := 10

		for iteration := 0; iteration < maxIterations; iteration++ {
			select {
			case <-ctx.Done(): return
			default:
			}

			msgs := a.contextBuilder.BuildMessages(sess)
			req := &provider.ChatRequest{
				Messages: msgs, Tools: providerTools, MaxTokens: 0, Temperature: 0, Stream: true,
			}

			if streamProvider, ok := p.(provider.StreamToolProvider); ok {
				done, err := a.handleNativeStreamTool(ctx, streamProvider, req, sess, resultChan, trace)
				if err != nil { resultChan <- StreamChunk{Type: "error", Error: err}; return }
				if done {
					if err := a.hook.AfterStream(ctx, sess); err != nil { logger.Warn("Hook AfterStream failed", logger.ErrorField(err)) }
					return
				}
			} else {
				done, err := a.handleFallbackStream(ctx, p, req, sess, resultChan, trace)
				if err != nil { resultChan <- StreamChunk{Type: "error", Error: err}; return }
				if done {
					if err := a.hook.AfterStream(ctx, sess); err != nil { logger.Warn("Hook AfterStream failed", logger.ErrorField(err)) }
					return
				}
			}
		}

		logger.Warn("Max tool iterations reached in stream")
		if err := a.hook.AfterStream(ctx, sess); err != nil { logger.Warn("Hook AfterStream failed", logger.ErrorField(err)) }
	}()

	return resultChan, nil
}

func (a *Agent) handleNativeStreamTool(
	ctx context.Context,
	p provider.StreamToolProvider,
	req *provider.ChatRequest,
	sess *session.Session,
	resultChan chan<- StreamChunk,
	trace *TraceSession,
) (bool, error) {

	stream, err := p.ChatStreamWithTools(ctx, req)
	if err != nil { return false, err }

	var fullContent string
	var currentToolCalls []provider.ToolCall
	var toolCallInProgress bool

	for chunk := range stream {
		select {
		case <-ctx.Done(): return true, ctx.Err()
		default:
		}

		if chunk.Error != nil { logger.Error("Stream error", logger.ErrorField(chunk.Error)); return false, chunk.Error }

		if chunk.Content != "" {
			fullContent += chunk.Content
			resultChan <- StreamChunk{Type: "text", Content: chunk.Content}
			if err := a.hook.OnStreamDelta(ctx, chunk.Content); err != nil { logger.Warn("Hook OnStreamDelta failed", logger.ErrorField(err)) }
		}

		if len(chunk.ToolCalls) > 0 && !toolCallInProgress {
			toolCallInProgress = true
			currentToolCalls = append(currentToolCalls, chunk.ToolCalls...)
			resultChan <- StreamChunk{Type: "tool_start", ToolCalls: chunk.ToolCalls}
		}

		if chunk.FinishReason != "" {
			logger.Debug("Stream finished", logger.String("reason", chunk.FinishReason))
			if len(currentToolCalls) > 0 {
				assistantMsg := session.NewAssistantMessage("", "", "", fullContent)
				if len(currentToolCalls) > 0 {
					assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(currentToolCalls))
					for _, tc := range currentToolCalls {
						assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
					}
				}
				sess.AddMessage(assistantMsg)

				toolResponses := a.executeToolCalls(ctx, currentToolCalls, trace)
				resultChan <- StreamChunk{Type: "tool_result", ToolResults: toolResponses}

				toolRespJSON, _ := json.Marshal(toolResponses)
				toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
				sess.AddMessage(toolMsg)

				return false, nil
			}

			if fullContent != "" {
				assistantMsg := session.NewAssistantMessage("", "", "", fullContent)
				sess.AddMessage(assistantMsg)
				if err := a.sessionMgr.SaveSession(sess); err != nil { logger.Warn("Failed to save session", logger.ErrorField(err)) }
				resultChan <- StreamChunk{Type: "end", Content: fullContent, SessionMsg: &assistantMsg}
			}
			return true, nil
		}
	}

	return true, nil
}

func (a *Agent) handleFallbackStream(
	ctx context.Context,
	p provider.Provider,
	req *provider.ChatRequest,
	sess *session.Session,
	resultChan chan<- StreamChunk,
	trace *TraceSession,
) (bool, error) {

	resp, err := a.callLLMWithRetry(ctx, p, req)
	if err != nil { return false, err }

	if resp.Content != "" {
		for _, c := range resp.Content {
			select {
			case <-ctx.Done(): return true, ctx.Err()
			case <-time.After(5 * time.Millisecond):
				resultChan <- StreamChunk{Type: "text", Content: string(c)}
				if err := a.hook.OnStreamDelta(ctx, string(c)); err != nil { logger.Warn("Hook OnStreamDelta failed", logger.ErrorField(err)) }
			}
		}
	}

	if len(resp.ToolCalls) > 0 && a.toolRegistry != nil {
		resultChan <- StreamChunk{Type: "tool_start", ToolCalls: resp.ToolCalls}

		assistantMsg := session.NewAssistantMessage("", "", "", resp.Content)
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
			}
		}
		sess.AddMessage(assistantMsg)

		toolResponses := a.executeToolCalls(ctx, resp.ToolCalls, trace)
		resultChan <- StreamChunk{Type: "tool_result", ToolResults: toolResponses}

		toolRespJSON, _ := json.Marshal(toolResponses)
		toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
		sess.AddMessage(toolMsg)
		return false, nil
	}

	assistantMsg := session.NewAssistantMessage("", "", "", resp.Content)
	sess.AddMessage(assistantMsg)
	if err := a.sessionMgr.SaveSession(sess); err != nil { logger.Warn("Failed to save session", logger.ErrorField(err)) }
	resultChan <- StreamChunk{Type: "end", Content: resp.Content, SessionMsg: &assistantMsg}
	return true, nil
}

func (a *Agent) Start() error {
	if a.msgBus == nil { return nil }
	a.msgBus.RegisterInboundHandler(a.handleInboundMessage)
	logger.Info("Agent started with message bus integration")
	return nil
}

func (a *Agent) handleInboundMessage(ctx context.Context, msg *bus.InboundMessage) error {
	logger.Debug("Agent received message from bus",
		logger.String("message_id", msg.ID),
		logger.String("channel_id", msg.ChannelID),
		logger.String("chat_id", msg.ChatID),
	)

	var sess *session.Session
	var err error

	if msg.Metadata != nil {
		if sessionID, ok := msg.Metadata["session_id"].(string); ok && sessionID != "" {
			sess, err = a.sessionMgr.GetSessionWithValidation(sessionID, msg.ChannelID, msg.UserID)
		}
	}

	if sess == nil || err != nil {
		sess, err = a.sessionMgr.CreateSession(msg.ChannelID, msg.UserID)
		if err != nil { return fmt.Errorf("failed to create session: %w", err) }
	}

	if a.msgBus != nil {
		startMsg := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, "")
		startMsg.Type = bus.MessageType("chat_start")
		startMsg.SessionID = sess.ID
		_ = a.msgBus.PublishOutbound(startMsg)
	}

	streamChan, err := a.ChatStream(ctx, sess, msg.Content)
	if err != nil {
		if a.msgBus != nil {
			errMsg := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, err.Error())
			errMsg.Type = bus.MessageType("error")
			errMsg.SessionID = sess.ID
			_ = a.msgBus.PublishOutbound(errMsg)
		}
		return err
	}

	var fullContent string
	for chunk := range streamChan {
		switch chunk.Type {
		case "text":
			fullContent += chunk.Content
			if a.msgBus != nil {
				deltaMsg := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, chunk.Content)
				deltaMsg.Type = bus.MessageType("chat_chunk")
				deltaMsg.SessionID = sess.ID
				_ = a.msgBus.PublishOutbound(deltaMsg)
			}
		case "tool_start":
			if a.msgBus != nil {
				toolMsg := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, "")
				toolMsg.Type = bus.MessageType("tool_call_start")
				toolMsg.SessionID = sess.ID
				toolMsg.Metadata = map[string]interface{}{"tool_calls": chunk.ToolCalls}
				_ = a.msgBus.PublishOutbound(toolMsg)
			}
		case "tool_result":
			if a.msgBus != nil {
				resultMsg := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, "")
				resultMsg.Type = bus.MessageType("tool_call_result")
				resultMsg.SessionID = sess.ID
				resultMsg.Metadata = map[string]interface{}{"tool_results": chunk.ToolResults}
				_ = a.msgBus.PublishOutbound(resultMsg)
			}
		case "end":
			if a.msgBus != nil {
				endMsg := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, fullContent)
				endMsg.Type = bus.MessageType("chat_end")
				endMsg.SessionID = sess.ID
				_ = a.msgBus.PublishOutbound(endMsg)
			}
		case "error":
			if a.msgBus != nil {
				errMsg := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, chunk.Error.Error())
				errMsg.Type = bus.MessageType("error")
				errMsg.SessionID = sess.ID
				_ = a.msgBus.PublishOutbound(errMsg)
			}
		}
	}

	return nil
}
