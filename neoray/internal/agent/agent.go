package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"neoray/internal/bus"
	"neoray/internal/command"
	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/memory"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/subagent"
	"neoray/internal/templates"
	"neoray/internal/tools"
)

// Agent AI 代理
type Agent struct {
	cfg                *config.Config
	providerMgr        *provider.ProviderManager
	sessionMgr         *session.Manager
	contextBuilder     *ContextBuilder
	toolRegistry       *tools.Registry
	tokenManager       *TokenManager
	traceManager       *TraceManager
	msgBus             *bus.MessageBus
	hook               AgentHook
	memoryManager      *memory.MemoryManager
	cmdManager         *command.Manager
	continuationMgr    *ContinuationManager
	subagentManager    *subagent.Manager
	spawnTool          *subagent.SpawnTool
	runtimeState       *tools.RuntimeState
	goalManager        *session.GoalManager
	currentSession     *session.Session
	fileStateStore     *tools.FileStateStore
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

// WithMemoryManager 设置记忆管理器
func WithMemoryManager(mgr *memory.MemoryManager) AgentOption {
	return func(a *Agent) {
		a.memoryManager = mgr
		// 更新 ContextBuilder 以包含记忆管理器
		if a.contextBuilder != nil {
			a.contextBuilder = NewContextBuilder(
				a.cfg,
				WithStrategy(a.contextBuilder.strategy),
				WithMemoryForContext(mgr),
			)
		}
	}
}

// WithContinuationManager 设置续轮管理器
func WithContinuationManager(cm *ContinuationManager) AgentOption {
	return func(a *Agent) {
		a.continuationMgr = cm
	}
}

// WithSubagentManager 设置子代理管理器
func WithSubagentManager(sm *subagent.Manager) AgentOption {
	return func(a *Agent) {
		a.subagentManager = sm
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
		toolRegistry:   toolRegistry,
	}

	// 创建指令管理器
	a.cmdManager = command.NewManager(cfg, providerMgr)

	// 先应用选项
	for _, opt := range opts {
		opt(a)
	}

	// 创建 runtime state
	a.runtimeState = tools.NewRuntimeState()

	// 创建 goal manager
	a.goalManager = session.NewGoalManager(sessionMgr, a.msgBus)

	// 创建 file state store
	a.fileStateStore = tools.NewFileStateStore()

	// 创建 ContextBuilder（可能包含记忆管理器）
	if a.memoryManager != nil {
		a.contextBuilder = &ContextBuilder{
			cfg:           cfg,
			strategy:      StrategyRecent,
			memoryManager: a.memoryManager,
		}
	} else {
		a.contextBuilder = NewContextBuilder(cfg)
	}

	// 默认值
	if a.tokenManager == nil { a.tokenManager = NewTokenManager(0) } // 无限制
	if a.traceManager == nil { a.traceManager = NewTraceManager(false) } // 默认禁用
	if a.hook == nil { a.hook = NewBaseHook("noop") } // 默认空 Hook
	if a.continuationMgr == nil { a.continuationMgr = NewContinuationManager() } // 默认启用续轮

	// 初始化子代理管理器（如果配置启用或提供了）
	if a.subagentManager == nil && a.toolRegistry != nil {
		// 检查是否启用子代理
		enabled := true
		if a.cfg != nil && a.cfg.Tools.Subagent.Enabled == false {
			enabled = false
		}

		if enabled {
			a.subagentManager = subagent.NewManager(
				a.cfg,
				a.providerMgr,
				a.msgBus,
				a.toolRegistry,
				a.memoryManager,
			)

			// 应用配置
			if a.cfg != nil {
				if a.cfg.Tools.Subagent.MaxConcurrent > 0 {
					a.subagentManager = a.subagentManager.WithMaxConcurrent(a.cfg.Tools.Subagent.MaxConcurrent)
				}
				if a.cfg.Tools.Subagent.MaxIterations > 0 {
					a.subagentManager = a.subagentManager.WithMaxIterations(a.cfg.Tools.Subagent.MaxIterations)
				}
				if a.cfg.Tools.Subagent.MaxToolResultChars > 0 {
					a.subagentManager = a.subagentManager.WithMaxToolResultChars(a.cfg.Tools.Subagent.MaxToolResultChars)
				}
			}

			// 创建并注册spawn工具
			a.spawnTool = subagent.NewSpawnTool(a.subagentManager)
			a.toolRegistry.Register(a.spawnTool)
		}
	}

	// 注册 reflection 工具（总是可用）
	reflectionTool := tools.NewReflectionTool(a.runtimeState, true)
	a.toolRegistry.Register(reflectionTool)

	// 注册 long_task 和 complete_goal 工具
	getSessionFunc := func() *session.Session {
		return a.currentSession
	}
	longTaskTool := tools.NewLongTaskTool(a.goalManager, getSessionFunc)
	completeGoalTool := tools.NewCompleteGoalTool(a.goalManager, getSessionFunc)
	a.toolRegistry.Register(longTaskTool)
	a.toolRegistry.Register(completeGoalTool)

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

	// 记录对话到记忆系统
	if a.memoryManager != nil && result != nil && result.Message != nil {
		// 查找最后的用户输入
		var userInput string
		for i := len(sess.Messages) - 1; i >= 0; i-- {
			if sess.Messages[i].Role == "user" {
				userInput = sess.Messages[i].Content
				break
			}
		}
		if userInput != "" && result.Message.Content != "" {
			if _, err := a.memoryManager.AppendHistory(userInput, result.Message.Content); err != nil {
				logger.Warn("Failed to append to history", logger.ErrorField(err))
			}
		}
	}

	return result
}

// Chat 发送聊天消息
func (a *Agent) Chat(ctx context.Context, sess *session.Session, userInput string) (*ChatResult, error) {
	startTime := time.Now()

	// 设置当前会话
	a.currentSession = sess

	// 为当前会话设置 FileStates
	if a.fileStateStore != nil {
		fileStates := a.fileStateStore.ForSession(sess.ID)
		// 更新已注册的文件系统工具和 patch 工具
		if tool, ok := a.toolRegistry.Get("filesystem"); ok {
			if fsTool, ok := tool.(*tools.FileSystemTool); ok {
				fsTool.SetFileStates(fileStates)
			}
		}
		if tool, ok := a.toolRegistry.Get("apply_patch"); ok {
			if patchTool, ok := tool.(*tools.ApplyPatchTool); ok {
				patchTool.SetFileStates(fileStates)
			}
		}
	}

	// 设置spawn工具的上下文（如果启用了子代理）
	if a.spawnTool != nil {
		a.spawnTool.SetOriginContext(
			"cli",       // 频道ID
			"direct",    // 聊天ID
			sess.ID,     // 会话ID
			"",          // 消息ID
		)
	}

	// 先检查是否是指令
	if a.cmdManager != nil {
		if resp, isCmd, err := a.cmdManager.Process(ctx, sess, userInput); isCmd {
			var assistantMsg session.Message
			if err != nil {
				assistantMsg = session.NewAssistantMessage("", "", "", fmt.Sprintf("❌ Command error: %v", err))
			} else {
				assistantMsg = session.NewAssistantMessage("", "", "", resp)
			}
			sess.AddMessage(assistantMsg)
			_ = a.sessionMgr.SaveSession(sess)

			result := &ChatResult{
				Message:  &assistantMsg,
				Duration: time.Since(startTime),
				Error:    err,
			}
			return a.finishChat(ctx, sess, result), nil
		}
	}

	// 跟踪活跃会话
	if a.memoryManager != nil {
		a.memoryManager.TrackSession(sess.ID)
	}

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

		// 检查是否需要续轮（仅在没有工具调用时才续轮）
		if len(resp.ToolCalls) == 0 || a.toolRegistry == nil {
			// 检查是否需要续轮
			if a.continuationMgr.IsTruncated(resp.FinishReason) {
				logger.Debug("Response truncated, starting continuation",
					logger.String("finish_reason", resp.FinishReason))

				// 执行续轮
				var continuationCount int
				var stillTruncated bool
				for continuationCount = 0; a.continuationMgr.ShouldContinue(continuationCount); continuationCount++ {
					contResp, truncated, contErr := a.continuationMgr.ExecuteContinuation(
						ctx, p, sess, providerTools, a.callLLMWithRetry)

					if contErr != nil {
						logger.Warn("Continuation failed, stopping", logger.ErrorField(contErr))
						break
					}

					// 合并续轮结果到最后一条消息
					a.continuationMgr.MergeContinuation(sess, contResp.Content)
					stillTruncated = truncated

					if !stillTruncated {
						logger.Debug("Continuation completed successfully",
							logger.Int("continuations", continuationCount+1))
						break
					}
				}

				if stillTruncated {
					logger.Warn("Still truncated after max continuations",
						logger.Int("continuations", continuationCount))
				}
			}

			// 保存并返回
			if err := a.sessionMgr.SaveSession(sess); err != nil { logger.Warn("Failed to save session", logger.ErrorField(err)) }
			finalMsg := sess.LastMessage()
			result := &ChatResult{
				Message: finalMsg,
				TokenUsage: a.tokenManager.GetSessionUsage(sess.ID),
				Trace: trace,
				ToolCalls: totalToolCalls,
				Iterations: iterations + 1,
				Duration: time.Since(startTime),
			}
			return a.finishChat(ctx, sess, result), nil
		}

		// 有工具调用，继续执行
		totalToolCalls += len(resp.ToolCalls)
		toolResponses := a.executeToolCalls(ctx, resp.ToolCalls, trace)
		toolRespJSON, _ := json.Marshal(toolResponses)
		toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
		sess.AddMessage(toolMsg)
	}

	logger.Warn("Max tool iterations reached")
	if trace != nil { trace.AddInfo("达到最大迭代次数", map[string]interface{}{"max_iterations": maxIterations}) }

	// 生成最大迭代次数的提示消息
	loader := templates.GetTemplateLoader()
	var maxIterMsg string
	if msg, err := loader.RenderTemplate("agent/max_iterations_message.md", map[string]string{
		"max_iterations": fmt.Sprintf("%d", maxIterations),
	}); err == nil {
		maxIterMsg = msg
	} else {
		maxIterMsg = fmt.Sprintf("I reached the maximum number of tool call iterations (%d) without completing the task. You can try breaking the task into smaller steps.", maxIterations)
	}

	// 添加助手消息
	assistantMsg := session.NewAssistantMessage("", "", "", maxIterMsg)
	sess.AddMessage(assistantMsg)

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

	// 设置当前会话
	a.currentSession = sess

	// 为当前会话设置 FileStates
	if a.fileStateStore != nil {
		fileStates := a.fileStateStore.ForSession(sess.ID)
		// 更新已注册的文件系统工具和 patch 工具
		if tool, ok := a.toolRegistry.Get("filesystem"); ok {
			if fsTool, ok := tool.(*tools.FileSystemTool); ok {
				fsTool.SetFileStates(fileStates)
			}
		}
		if tool, ok := a.toolRegistry.Get("apply_patch"); ok {
			if patchTool, ok := tool.(*tools.ApplyPatchTool); ok {
				patchTool.SetFileStates(fileStates)
			}
		}
	}

	// 设置spawn工具的上下文（如果启用了子代理）
	if a.spawnTool != nil {
		a.spawnTool.SetOriginContext(
			"cli",       // 频道ID
			"direct",    // 聊天ID
			sess.ID,     // 会话ID
			"",          // 消息ID
		)
	}

	// 先检查是否是指令
	if a.cmdManager != nil {
		if resp, isCmd, err := a.cmdManager.Process(ctx, sess, userInput); isCmd {
			go func() {
				defer close(resultChan)
				var assistantMsg session.Message
				if err != nil {
					assistantMsg = session.NewAssistantMessage("", "", "", fmt.Sprintf("❌ Command error: %v", err))
				} else {
					assistantMsg = session.NewAssistantMessage("", "", "", resp)
				}
				sess.AddMessage(assistantMsg)
				_ = a.sessionMgr.SaveSession(sess)
				resultChan <- StreamChunk{Type: "end", Content: resp, SessionMsg: &assistantMsg}
			}()
			return resultChan, nil
		}
	}

	// 跟踪活跃会话
	if a.memoryManager != nil {
		a.memoryManager.TrackSession(sess.ID)
	}

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
				done, err := a.handleNativeStreamTool(ctx, streamProvider, req, sess, resultChan, trace, providerTools)
				if err != nil { resultChan <- StreamChunk{Type: "error", Error: err}; return }
				if done {
					if err := a.hook.AfterStream(ctx, sess); err != nil { logger.Warn("Hook AfterStream failed", logger.ErrorField(err)) }
					return
				}
			} else {
				done, err := a.handleFallbackStream(ctx, p, req, sess, resultChan, trace, providerTools)
				if err != nil { resultChan <- StreamChunk{Type: "error", Error: err}; return }
				if done {
					if err := a.hook.AfterStream(ctx, sess); err != nil { logger.Warn("Hook AfterStream failed", logger.ErrorField(err)) }
					return
				}
			}
		}

		logger.Warn("Max tool iterations reached in stream")
		// 生成最大迭代次数的提示消息
		loader := templates.GetTemplateLoader()
		var maxIterMsg string
		if msg, err := loader.RenderTemplate("agent/max_iterations_message.md", map[string]string{
			"max_iterations": fmt.Sprintf("%d", maxIterations),
		}); err == nil {
			maxIterMsg = msg
		} else {
			maxIterMsg = fmt.Sprintf("I reached the maximum number of tool call iterations (%d) without completing the task. You can try breaking the task into smaller steps.", maxIterations)
		}
		// 添加助手消息
		assistantMsg := session.NewAssistantMessage("", "", "", maxIterMsg)
		sess.AddMessage(assistantMsg)
		// 发送给用户
		resultChan <- StreamChunk{Type: "text", Content: maxIterMsg}
		resultChan <- StreamChunk{Type: "end", Content: maxIterMsg, SessionMsg: &assistantMsg}
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
	providerTools []provider.Tool,
) (bool, error) {

	stream, err := p.ChatStreamWithTools(ctx, req)
	if err != nil { return false, err }

	var fullContent string
	var currentToolCalls []provider.ToolCall
	var toolCallInProgress bool
	var finishReason string

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
			finishReason = chunk.FinishReason
			logger.Debug("Stream finished", logger.String("reason", finishReason))
			break
		}
	}

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

		// 检查是否需要续轮
		if a.continuationMgr.IsTruncated(finishReason) {
			logger.Debug("Stream response truncated, starting continuation",
				logger.String("finish_reason", finishReason))

			// 执行续轮
			var continuationCount int
			var stillTruncated bool
			for continuationCount = 0; a.continuationMgr.ShouldContinue(continuationCount); continuationCount++ {
				contResp, truncated, contErr := a.continuationMgr.ExecuteContinuation(
					ctx, p, sess, providerTools, a.callLLMWithRetry)

				if contErr != nil {
					logger.Warn("Stream continuation failed, stopping", logger.ErrorField(contErr))
					break
				}

				// 流式输出续轮内容
				for _, c := range contResp.Content {
					select {
					case <-ctx.Done(): return true, ctx.Err()
					case <-time.After(5 * time.Millisecond):
						resultChan <- StreamChunk{Type: "text", Content: string(c)}
						if err := a.hook.OnStreamDelta(ctx, string(c)); err != nil { logger.Warn("Hook OnStreamDelta failed", logger.ErrorField(err)) }
					}
				}

				// 合并到消息
				a.continuationMgr.MergeContinuation(sess, contResp.Content)
				stillTruncated = truncated

				if !stillTruncated {
					logger.Debug("Stream continuation completed successfully",
						logger.Int("continuations", continuationCount+1))
					break
				}
			}

			if stillTruncated {
				logger.Warn("Still truncated after max continuations in stream",
					logger.Int("continuations", continuationCount))
			}

			// 更新 finalMsg
			assistantMsg = sess.Messages[len(sess.Messages)-1]
		}

		if err := a.sessionMgr.SaveSession(sess); err != nil { logger.Warn("Failed to save session", logger.ErrorField(err)) }
		resultChan <- StreamChunk{Type: "end", Content: assistantMsg.Content, SessionMsg: &assistantMsg}
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
	providerTools []provider.Tool,
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

	// 检查是否需要续轮
	if a.continuationMgr.IsTruncated(resp.FinishReason) {
		logger.Debug("Fallback stream truncated, starting continuation",
			logger.String("finish_reason", resp.FinishReason))

		// 执行续轮
		var continuationCount int
		var stillTruncated bool
		for continuationCount = 0; a.continuationMgr.ShouldContinue(continuationCount); continuationCount++ {
			contResp, truncated, contErr := a.continuationMgr.ExecuteContinuation(
				ctx, p, sess, providerTools, a.callLLMWithRetry)

			if contErr != nil {
				logger.Warn("Fallback continuation failed, stopping", logger.ErrorField(contErr))
				break
			}

			// 流式输出续轮内容
			for _, c := range contResp.Content {
				select {
				case <-ctx.Done(): return true, ctx.Err()
				case <-time.After(5 * time.Millisecond):
					resultChan <- StreamChunk{Type: "text", Content: string(c)}
					if err := a.hook.OnStreamDelta(ctx, string(c)); err != nil { logger.Warn("Hook OnStreamDelta failed", logger.ErrorField(err)) }
				}
			}

			// 合并到消息
			a.continuationMgr.MergeContinuation(sess, contResp.Content)
			stillTruncated = truncated

			if !stillTruncated {
				logger.Debug("Fallback continuation completed successfully",
					logger.Int("continuations", continuationCount+1))
				break
			}
		}

		if stillTruncated {
			logger.Warn("Still truncated after max continuations in fallback stream",
				logger.Int("continuations", continuationCount))
		}

		// 更新 finalMsg
		assistantMsg = sess.Messages[len(sess.Messages)-1]
	}

	if err := a.sessionMgr.SaveSession(sess); err != nil { logger.Warn("Failed to save session", logger.ErrorField(err)) }
	resultChan <- StreamChunk{Type: "end", Content: assistantMsg.Content, SessionMsg: &assistantMsg}
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

	// 设置spawn工具的上下文（如果启用了子代理）
	if a.spawnTool != nil {
		sessionKey := fmt.Sprintf("%s:%s", msg.ChannelID, msg.ChatID)
		if msg.Metadata != nil {
			if sk, ok := msg.Metadata["session_key_override"].(string); ok {
				sessionKey = sk
			}
		}
		a.spawnTool.SetOriginContext(
			msg.ChannelID, // 频道ID
			msg.ChatID,    // 聊天ID
			sessionKey,    // 会话ID
			msg.ID,        // 消息ID
		)
	}

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
