package agent

import (
	"context"
	"encoding/json"
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

// TurnState 表示代理处理消息的状态
type TurnState string

const (
	// TurnStateRestore 恢复会话状态
	TurnStateRestore TurnState = "restore"
	// TurnStateCompact 压缩会话历史
	TurnStateCompact TurnState = "compact"
	// TurnStateCommand 处理命令
	TurnStateCommand TurnState = "command"
	// TurnStateBuild 构建上下文
	TurnStateBuild TurnState = "build"
	// TurnStateRun 运行 LLM
	TurnStateRun TurnState = "run"
	// TurnStateSave 保存会话
	TurnStateSave TurnState = "save"
	// TurnStateRespond 发送响应
	TurnStateRespond TurnState = "respond"
	// TurnStateDone 完成处理
	TurnStateDone TurnState = "done"
)

// StateEvent 表示状态处理后的事件
type StateEvent string

const (
	// StateEventOK 状态处理成功，继续正常流程
	StateEventOK StateEvent = "ok"
	// StateEventDispatch 命令已分发，继续正常流程
	StateEventDispatch StateEvent = "dispatch"
	// StateEventShortcut 命令已处理，跳转到完成
	StateEventShortcut StateEvent = "shortcut"
)

// StateTraceEntry 状态追踪记录
type StateTraceEntry struct {
	State     TurnState
	StartedAt time.Time
	Duration  time.Duration
	Event     StateEvent
	Error     error
}

// TurnContext 回合上下文
type TurnContext struct {
	// 基本信息
	Msg        *bus.InboundMessage
	SessionKey string
	State      TurnState
	TurnID     string
	Session    *session.Session

	// 消息内容
	History         []map[string]interface{}
	InitialMessages []provider.Message
	FinalContent    string
	ToolsUsed       []string
	AllMessages     []provider.Message
	StopReason      string
	HadInjections   bool

	// 控制标志
	UserPersistedEarly bool
	SuppressResponse   bool

	// 输出
	Outbound *bus.OutboundMessage

	// 回调
	OnProgress    func(ctx context.Context, data map[string]interface{}) error
	OnStream      func(ctx context.Context, delta string) error
	OnStreamEnd   func(ctx context.Context, resuming bool) error
	OnRetryWait   func(ctx context.Context, message string) error

	// 队列
	PendingQueue chan *bus.InboundMessage

	// 时间
	TurnWallStartedAt  time.Time
	VisibleRunStartedAt *time.Time
	TurnLatencyMs      *int64

	// 追踪
	Trace []StateTraceEntry

	// 内部状态
	saveSkip int
}

// NewTurnContext 创建新的回合上下文
func NewTurnContext(msg *bus.InboundMessage, sessionKey string) *TurnContext {
	return &TurnContext{
		Msg:              msg,
		SessionKey:       sessionKey,
		State:            TurnStateRestore,
		TurnID:           fmt.Sprintf("%s:%d", sessionKey, time.Now().UnixNano()),
		TurnWallStartedAt: time.Now(),
		Trace:            make([]StateTraceEntry, 0),
	}
}

// AgentLoop 代理循环
type AgentLoop struct {
	cfg             *config.Config
	providerMgr     *provider.ProviderManager
	sessionMgr      *session.Manager
	contextBuilder  *ContextBuilder
	toolRegistry    *tools.Registry
	tokenManager    *TokenManager
	traceManager    *TraceManager
	msgBus          *bus.MessageBus
	hook            AgentHook
	memoryManager   *memory.MemoryManager
	cmdManager      *command.Manager
	continuationMgr *ContinuationManager
	subagentManager *subagent.Manager
	spawnTool       *subagent.SpawnTool

	// 状态转换表
	transitions map[TurnState]map[StateEvent]TurnState

	// 并发控制
	running         bool
	activeTasks     map[string][]func()
	backgroundTasks []func()
	sessionLocks    map[string]*sync.Mutex
	pendingQueues   map[string]chan *bus.InboundMessage
	concurrencyGate chan struct{}

	mu sync.RWMutex
}

// AgentLoopOption 代理循环配置选项
type AgentLoopOption func(*AgentLoop)

// WithTokenManagerForLoop 设置 Token 管理器
func WithTokenManagerForLoop(tm *TokenManager) AgentLoopOption {
	return func(al *AgentLoop) {
		al.tokenManager = tm
	}
}

// WithTraceManagerForLoop 设置追踪管理器
func WithTraceManagerForLoop(tm *TraceManager) AgentLoopOption {
	return func(al *AgentLoop) {
		al.traceManager = tm
	}
}

// WithMessageBusForLoop 设置消息总线
func WithMessageBusForLoop(mb *bus.MessageBus) AgentLoopOption {
	return func(al *AgentLoop) {
		al.msgBus = mb
	}
}

// WithHookForLoop 设置 Agent Hook
func WithHookForLoop(hook AgentHook) AgentLoopOption {
	return func(al *AgentLoop) {
		al.hook = hook
	}
}

// WithMemoryManagerForLoop 设置记忆管理器
func WithMemoryManagerForLoop(mgr *memory.MemoryManager) AgentLoopOption {
	return func(al *AgentLoop) {
		al.memoryManager = mgr
	}
}

// WithContinuationManagerForLoop 设置续轮管理器
func WithContinuationManagerForLoop(cm *ContinuationManager) AgentLoopOption {
	return func(al *AgentLoop) {
		al.continuationMgr = cm
	}
}

// WithSubagentManagerForLoop 设置子代理管理器
func WithSubagentManagerForLoop(sm *subagent.Manager) AgentLoopOption {
	return func(al *AgentLoop) {
		al.subagentManager = sm
	}
}

// NewAgentLoop 创建代理循环
func NewAgentLoop(
	cfg *config.Config,
	providerMgr *provider.ProviderManager,
	sessionMgr *session.Manager,
	toolRegistry *tools.Registry,
	opts ...AgentLoopOption,
) *AgentLoop {
	al := &AgentLoop{
		cfg:             cfg,
		providerMgr:     providerMgr,
		sessionMgr:      sessionMgr,
		toolRegistry:    toolRegistry,
		activeTasks:     make(map[string][]func()),
		sessionLocks:    make(map[string]*sync.Mutex),
		pendingQueues:   make(map[string]chan *bus.InboundMessage),
	}

	// 创建指令管理器
	al.cmdManager = command.NewManager(cfg, providerMgr)

	// 应用选项
	for _, opt := range opts {
		opt(al)
	}

	// 创建 ContextBuilder
	if al.memoryManager != nil {
		al.contextBuilder = &ContextBuilder{
			cfg:             cfg,
			strategy:        StrategyRecent,
			memoryManager:   al.memoryManager,
		}
	} else {
		al.contextBuilder = NewContextBuilder(cfg)
	}

	// 默认值
	if al.tokenManager == nil {
		al.tokenManager = NewTokenManager(0) // 无限制
	}
	if al.traceManager == nil {
		al.traceManager = NewTraceManager(false) // 默认禁用
	}
	if al.hook == nil {
		al.hook = NewBaseHook("noop") // 默认空 Hook
	}
	if al.continuationMgr == nil {
		al.continuationMgr = NewContinuationManager() // 默认启用续轮
	}

	// 初始化子代理管理器
	if al.subagentManager == nil && al.toolRegistry != nil {
		enabled := true
		if al.cfg != nil && al.cfg.Tools.Subagent.Enabled == false {
			enabled = false
		}

		if enabled {
			al.subagentManager = subagent.NewManager(
				al.cfg,
				al.providerMgr,
				al.msgBus,
				al.toolRegistry,
				al.memoryManager,
			)

			if al.cfg != nil {
				if al.cfg.Tools.Subagent.MaxConcurrent > 0 {
					al.subagentManager = al.subagentManager.WithMaxConcurrent(al.cfg.Tools.Subagent.MaxConcurrent)
				}
				if al.cfg.Tools.Subagent.MaxIterations > 0 {
					al.subagentManager = al.subagentManager.WithMaxIterations(al.cfg.Tools.Subagent.MaxIterations)
				}
				if al.cfg.Tools.Subagent.MaxToolResultChars > 0 {
					al.subagentManager = al.subagentManager.WithMaxToolResultChars(al.cfg.Tools.Subagent.MaxToolResultChars)
				}
			}

			al.spawnTool = subagent.NewSpawnTool(al.subagentManager)
			al.toolRegistry.Register(al.spawnTool)
		}
	}

	// 初始化状态转换表
	al.transitions = map[TurnState]map[StateEvent]TurnState{
		TurnStateRestore: {
			StateEventOK: TurnStateCompact,
		},
		TurnStateCompact: {
			StateEventOK: TurnStateCommand,
		},
		TurnStateCommand: {
			StateEventDispatch: TurnStateBuild,
			StateEventShortcut: TurnStateDone,
		},
		TurnStateBuild: {
			StateEventOK: TurnStateRun,
		},
		TurnStateRun: {
			StateEventOK: TurnStateSave,
		},
		TurnStateSave: {
			StateEventOK: TurnStateRespond,
		},
		TurnStateRespond: {
			StateEventOK: TurnStateDone,
		},
	}

	return al
}

// Start 启动代理循环
func (al *AgentLoop) Start() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.running {
		return nil
	}

	al.running = true

	if al.msgBus != nil {
		al.msgBus.RegisterInboundHandler(al.handleInboundMessage)
		logger.Info("Agent loop started with message bus integration")
	} else {
		logger.Info("Agent loop started (no message bus)")
	}

	return nil
}

// Stop 停止代理循环
func (al *AgentLoop) Stop() {
	al.mu.Lock()
	defer al.mu.Unlock()

	if !al.running {
		return
	}

	al.running = false
	logger.Info("Agent loop stopping")
}

// ProcessDirect 直接处理消息
func (al *AgentLoop) ProcessDirect(
	ctx context.Context,
	content string,
	sessionKey string,
	channelID string,
	chatID string,
) (*bus.OutboundMessage, error) {
	msg := &bus.InboundMessage{
		ID:        fmt.Sprintf("direct:%d", time.Now().UnixNano()),
		ChannelID: channelID,
		ChatID:    chatID,
		UserID:    "direct",
		Content:   content,
		Metadata:  make(map[string]interface{}),
	}

	lock := al.getSessionLock(sessionKey)
	lock.Lock()
	defer lock.Unlock()

	return al.processMessage(ctx, msg, sessionKey)
}

// handleInboundMessage 处理来自总线的消息
func (al *AgentLoop) handleInboundMessage(ctx context.Context, msg *bus.InboundMessage) error {
	sessionKey := al.effectiveSessionKey(msg)
	if sessionKey != msg.SessionKey() {
		msg = msg.WithSessionKeyOverride(sessionKey)
	}

	lock := al.getSessionLock(sessionKey)

	// 检查是否有 pending 队列
	al.mu.RLock()
	_, hasPending := al.pendingQueues[sessionKey]
	al.mu.RUnlock()

	if hasPending && al.cmdManager != nil {
		raw := msg.Content
		if al.cmdManager.IsDispatchableCommand(raw) {
			return al.dispatchCommandInline(ctx, msg, sessionKey, raw)
		}
	}

	// 启动新任务处理 — 派生独立 context，避免捕获短生命周期的请求 context
	go func() {
		lock.Lock()
		defer lock.Unlock()

		// 创建 pending 队列
		pendingQueue := make(chan *bus.InboundMessage, 20)
		al.mu.Lock()
		al.pendingQueues[sessionKey] = pendingQueue
		al.mu.Unlock()

		defer func() {
			al.mu.Lock()
			delete(al.pendingQueues, sessionKey)
			al.mu.Unlock()

			// 重新发布剩余的消息
			for {
				select {
				case leftoverMsg := <-pendingQueue:
					if al.msgBus != nil {
						_ = al.msgBus.PublishInbound(leftoverMsg)
					}
				default:
					return
				}
			}
		}()

		// Derive a background context so the goroutine survives the caller's scope.
		// Preserve logger/provider values but drop the caller's cancellation.
		goroutineCtx := context.WithoutCancel(ctx)
		const goroutineTimeout = 10 * time.Minute
		var cancel context.CancelFunc
		goroutineCtx, cancel = context.WithTimeout(goroutineCtx, goroutineTimeout)
		defer cancel()

		_, err := al.processMessage(goroutineCtx, msg, sessionKey)
		if err != nil {
			logger.Error("Error processing message", logger.ErrorField(err))
		}
	}()

	return nil
}

// processMessage 处理单条消息（状态机主循环）
func (al *AgentLoop) processMessage(
	ctx context.Context,
	msg *bus.InboundMessage,
	sessionKey string,
) (*bus.OutboundMessage, error) {
	ctx = al.refreshProviderSnapshot(ctx)

	if msg.ChannelID == "system" {
		return al.processSystemMessage(ctx, msg, sessionKey)
	}

	turnCtx := NewTurnContext(msg, sessionKey)

	// 状态机主循环
	for turnCtx.State != TurnStateDone {
		startedAt := time.Now()
		var event StateEvent
		var err error

		switch turnCtx.State {
		case TurnStateRestore:
			event, err = al.stateRestore(ctx, turnCtx)
		case TurnStateCompact:
			event, err = al.stateCompact(ctx, turnCtx)
		case TurnStateCommand:
			event, err = al.stateCommand(ctx, turnCtx)
		case TurnStateBuild:
			event, err = al.stateBuild(ctx, turnCtx)
		case TurnStateRun:
			event, err = al.stateRun(ctx, turnCtx)
		case TurnStateSave:
			event, err = al.stateSave(ctx, turnCtx)
		case TurnStateRespond:
			event, err = al.stateRespond(ctx, turnCtx)
		default:
			return nil, fmt.Errorf("unknown state: %s", turnCtx.State)
		}

		duration := time.Since(startedAt)

		// 记录追踪
		entry := StateTraceEntry{
			State:     turnCtx.State,
			StartedAt: startedAt,
			Duration:  duration,
			Event:     event,
			Error:     err,
		}
		turnCtx.Trace = append(turnCtx.Trace, entry)

		if err != nil {
			logger.Error("State handler failed",
				logger.String("state", string(turnCtx.State)),
				logger.ErrorField(err))
			return nil, err
		}

		logger.Debug("State transition",
			logger.String("turn_id", turnCtx.TurnID),
			logger.String("state", string(turnCtx.State)),
			logger.String("event", string(event)),
			logger.Duration("duration", duration))

		// 状态转换
		nextState, ok := al.transitions[turnCtx.State][event]
		if !ok {
			return nil, fmt.Errorf("no transition from state %s with event %s", turnCtx.State, event)
		}
		turnCtx.State = nextState
	}

	logger.Debug("Turn completed",
		logger.String("turn_id", turnCtx.TurnID),
		logger.Int("states", len(turnCtx.Trace)))

	return turnCtx.Outbound, nil
}

// stateRestore 恢复会话状态
func (al *AgentLoop) stateRestore(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Restore", logger.String("session_key", turnCtx.SessionKey))

	if turnCtx.Session == nil {
		var err error
		turnCtx.Session, err = al.sessionMgr.GetSession(turnCtx.SessionKey)
		if err != nil {
			turnCtx.Session, err = al.sessionMgr.CreateSession(
				turnCtx.Msg.ChannelID,
				turnCtx.Msg.UserID,
			)
			if err != nil {
				return StateEventOK, fmt.Errorf("failed to create session: %w", err)
			}
		}
	}

	// TODO: 恢复运行时检查点
	// TODO: 恢复待处理用户回合
	// TODO: 提取文档

	return StateEventOK, nil
}

// stateCompact 压缩会话历史
func (al *AgentLoop) stateCompact(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Compact", logger.String("session_key", turnCtx.SessionKey))

	// TODO: 自动压缩逻辑

	return StateEventOK, nil
}

// stateCommand 处理命令
func (al *AgentLoop) stateCommand(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Command", logger.String("session_key", turnCtx.SessionKey))

	if al.cmdManager == nil {
		return StateEventDispatch, nil
	}

	raw := turnCtx.Msg.Content
	resp, isCmd, err := al.cmdManager.Process(ctx, turnCtx.Session, raw)
	if !isCmd {
		return StateEventDispatch, nil
	}

	// 命令已处理
	if err != nil {
		turnCtx.Outbound = bus.NewOutboundMessage(
			turnCtx.Msg.ChannelID,
			turnCtx.Msg.ChatID,
			fmt.Sprintf("❌ Command error: %v", err),
		)
	} else {
		turnCtx.Outbound = bus.NewOutboundMessage(
			turnCtx.Msg.ChannelID,
			turnCtx.Msg.ChatID,
			resp,
		)
	}

	// 保存用户消息和命令响应（除了 /new）
	if raw != "/new" && raw != "/new " {
		turnCtx.UserPersistedEarly = true
		userMsg := session.NewUserMessage("", "", "", raw)
		turnCtx.Session.AddMessage(userMsg)

		assistantMsg := session.NewAssistantMessage("", "", "", turnCtx.Outbound.Content)
		turnCtx.Session.AddMessage(assistantMsg)

		if saveErr := al.sessionMgr.SaveSession(turnCtx.Session); saveErr != nil {
			logger.Error("Failed to save session", logger.ErrorField(saveErr))
		}
	}

	return StateEventShortcut, nil
}

// stateBuild 构建上下文
func (al *AgentLoop) stateBuild(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Build", logger.String("session_key", turnCtx.SessionKey))

	// 设置工具上下文
	if al.spawnTool != nil {
		al.spawnTool.SetOriginContext(
			turnCtx.Msg.ChannelID,
			turnCtx.Msg.ChatID,
			turnCtx.SessionKey,
			turnCtx.Msg.ID,
		)
	}

	// 构建初始消息
	turnCtx.InitialMessages = al.contextBuilder.BuildMessages(turnCtx.Session)

	// 持久化用户消息
	if !turnCtx.UserPersistedEarly {
		userMsg := session.NewUserMessage("", "", "", turnCtx.Msg.Content)
		turnCtx.Session.AddMessage(userMsg)
		turnCtx.UserPersistedEarly = true
	}

	// TODO: 可能的压缩

	return StateEventOK, nil
}

// stateRun 运行 LLM
func (al *AgentLoop) stateRun(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Run", logger.String("session_key", turnCtx.SessionKey))

	if turnCtx.VisibleRunStartedAt == nil {
		now := time.Now()
		turnCtx.VisibleRunStartedAt = &now
	}

	p, err := al.providerMgr.GetProvider(al.cfg.LLM.DefaultProvider)
	if err != nil || p == nil {
		p = al.providerMgr.DefaultProvider()
	}

	if p == nil {
		errMsg := "⚠️ No LLM provider configured! Please edit your config.toml and add an API key for Anthropic or OpenAI."
		logger.Error("No LLM provider available", logger.String("default_provider", al.cfg.LLM.DefaultProvider))
		turnCtx.FinalContent = errMsg
		return StateEventOK, nil
	}

	var providerTools []provider.Tool
	if al.toolRegistry != nil {
		for _, def := range al.toolRegistry.GetDefinitions() {
			var schema map[string]interface{}
			_ = json.Unmarshal(def.InputSchema, &schema)
			providerTools = append(providerTools, provider.Tool{
				Name:        def.Name,
				Description: def.Description,
				InputSchema: schema,
			})
		}
	}

	maxIterations := 10
	if al.cfg.Agent.MaxIterations > 0 {
		maxIterations = al.cfg.Agent.MaxIterations
	}

	var totalToolCalls int
	var iterations int

	for iterations = 0; iterations < maxIterations; iterations++ {
		select {
		case <-ctx.Done():
			return StateEventOK, ctx.Err()
		default:
		}

		msgs := turnCtx.InitialMessages
		if iterations > 0 {
			msgs = al.contextBuilder.BuildMessages(turnCtx.Session)
		}

		req := &provider.ChatRequest{
			Messages: msgs,
			Tools:    providerTools,
		}

		if providerCfg, ok := al.cfg.LLM.Providers[p.Name()]; ok {
			req.MaxTokens = providerCfg.MaxTokens
			req.Temperature = providerCfg.Temperature
		}

		logger.Debug("Calling LLM",
			logger.String("session_key", turnCtx.SessionKey),
			logger.String("provider", p.Name()),
			logger.Int("iteration", iterations+1))

		resp, err := al.callLLMWithRetry(ctx, p, req)
		if err != nil {
			logger.Error("LLM call failed after retries", logger.ErrorField(err), logger.Int("iteration", iterations+1))
			errMsg := fmt.Sprintf("I'm having trouble connecting to the AI service right now. Error: %v", err)
			turnCtx.FinalContent = errMsg
			return StateEventOK, nil
		}

		assistantMsg := session.NewAssistantMessage("", "", "", resp.Content)
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
		turnCtx.Session.AddMessage(assistantMsg)

		// 检查是否需要续轮（仅在没有工具调用时）
		if len(resp.ToolCalls) == 0 || al.toolRegistry == nil {
			if al.continuationMgr.IsTruncated(resp.FinishReason) {
				logger.Debug("Response truncated, starting continuation",
					logger.String("finish_reason", resp.FinishReason))

				var continuationCount int
				var stillTruncated bool
				for continuationCount = 0; al.continuationMgr.ShouldContinue(continuationCount); continuationCount++ {
					contResp, truncated, contErr := al.continuationMgr.ExecuteContinuation(
						ctx, p, turnCtx.Session, providerTools, al.callLLMWithRetry)

					if contErr != nil {
						logger.Warn("Continuation failed, stopping", logger.ErrorField(contErr))
						break
					}

					al.continuationMgr.MergeContinuation(turnCtx.Session, contResp.Content)
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

			// 获取最终内容
			finalMsg := turnCtx.Session.LastMessage()
			if finalMsg != nil {
				turnCtx.FinalContent = finalMsg.Content
			}
			turnCtx.StopReason = resp.FinishReason
			turnCtx.HadInjections = false
			return StateEventOK, nil
		}

		// 有工具调用，继续执行
		totalToolCalls += len(resp.ToolCalls)
		toolResponses := al.executeToolCalls(ctx, resp.ToolCalls, nil)
		toolRespJSON, _ := json.Marshal(toolResponses)
		toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
		turnCtx.Session.AddMessage(toolMsg)
	}

	logger.Warn("Max tool iterations reached")

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

	assistantMsg := session.NewAssistantMessage("", "", "", maxIterMsg)
	turnCtx.Session.AddMessage(assistantMsg)
	turnCtx.FinalContent = maxIterMsg

	return StateEventOK, nil
}

// stateSave 保存会话
func (al *AgentLoop) stateSave(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Save", logger.String("session_key", turnCtx.SessionKey))

	if turnCtx.FinalContent == "" || turnCtx.FinalContent == " " {
		turnCtx.FinalContent = " " // 空响应占位符
	}

	// 计算延迟
	latencyStartedAt := turnCtx.TurnWallStartedAt
	if turnCtx.VisibleRunStartedAt != nil {
		latencyStartedAt = *turnCtx.VisibleRunStartedAt
	}
	latencyMs := int64(time.Since(latencyStartedAt).Milliseconds())
	turnCtx.TurnLatencyMs = &latencyMs

	// 保存会话
	if err := al.sessionMgr.SaveSession(turnCtx.Session); err != nil {
		logger.Warn("Failed to save session", logger.ErrorField(err))
	}

	// TODO: 可能的后台压缩

	return StateEventOK, nil
}

// stateRespond 发送响应
func (al *AgentLoop) stateRespond(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Respond", logger.String("session_key", turnCtx.SessionKey))

	if turnCtx.SuppressResponse {
		turnCtx.Outbound = nil
		return StateEventOK, nil
	}

	turnCtx.Outbound = bus.NewOutboundMessage(
		turnCtx.Msg.ChannelID,
		turnCtx.Msg.ChatID,
		turnCtx.FinalContent,
	)
	turnCtx.Outbound.SessionID = turnCtx.SessionKey

	if turnCtx.TurnLatencyMs != nil {
		if turnCtx.Outbound.Metadata == nil {
			turnCtx.Outbound.Metadata = make(map[string]interface{})
		}
		turnCtx.Outbound.Metadata["latency_ms"] = *turnCtx.TurnLatencyMs
	}

	return StateEventOK, nil
}

// processSystemMessage 处理系统消息
func (al *AgentLoop) processSystemMessage(
	ctx context.Context,
	msg *bus.InboundMessage,
	sessionKey string,
) (*bus.OutboundMessage, error) {
	logger.Debug("Processing system message", logger.String("sender_id", msg.UserID))

	_, err := al.sessionMgr.GetSession(sessionKey)
	if err != nil {
		_, err = al.sessionMgr.CreateSession(msg.ChannelID, msg.UserID)
		if err != nil {
			return nil, err
		}
	}

	// TODO: 系统消息处理逻辑

	content := "System message processed"
	outbound := bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, content)
	return outbound, nil
}

// dispatchCommandInline 内联分发命令
func (al *AgentLoop) dispatchCommandInline(
	ctx context.Context,
	msg *bus.InboundMessage,
	sessionKey string,
	raw string,
) error {
	sess, err := al.sessionMgr.GetSession(sessionKey)
	if err != nil {
		sess, err = al.sessionMgr.CreateSession(msg.ChannelID, msg.UserID)
		if err != nil {
			return err
		}
	}

	resp, isCmd, err := al.cmdManager.Process(ctx, sess, raw)
	if !isCmd {
		return nil
	}

	var outboundMsg *bus.OutboundMessage
	if err != nil {
		outboundMsg = bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, fmt.Sprintf("❌ Command error: %v", err))
	} else {
		outboundMsg = bus.NewOutboundMessage(msg.ChannelID, msg.ChatID, resp)
	}

	if al.msgBus != nil {
		return al.msgBus.PublishOutbound(outboundMsg)
	}

	return nil
}

// effectiveSessionKey 获取有效的会话键
func (al *AgentLoop) effectiveSessionKey(msg *bus.InboundMessage) string {
	if al.cfg.Agent.UnifiedSession && msg.SessionKeyOverride() == "" {
		return "unified:default"
	}
	return msg.SessionKey()
}

// getSessionLock 获取会话锁
func (al *AgentLoop) getSessionLock(sessionKey string) *sync.Mutex {
	al.mu.Lock()
	defer al.mu.Unlock()

	lock, ok := al.sessionLocks[sessionKey]
	if !ok {
		lock = &sync.Mutex{}
		al.sessionLocks[sessionKey] = lock
	}
	return lock
}

// refreshProviderSnapshot 刷新提供者快照
func (al *AgentLoop) refreshProviderSnapshot(ctx context.Context) context.Context {
	// TODO: 实现提供者快照刷新逻辑
	return ctx
}

// callLLMWithRetry 带重试的 LLM 调用
func (al *AgentLoop) callLLMWithRetry(
	ctx context.Context,
	p provider.Provider,
	req *provider.ChatRequest,
) (*provider.ChatResponse, error) {
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
		logger.Warn("LLM call failed, retrying", logger.ErrorField(err), logger.Int("attempt", attempt+1))

		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<attempt)
			logger.Debug("Waiting before retry", logger.Duration("delay", delay))
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
func (al *AgentLoop) executeToolCalls(
	ctx context.Context,
	toolCalls []provider.ToolCall,
	trace *TraceSession,
) []map[string]interface{} {
	var toolResponses []map[string]interface{}
	var wg sync.WaitGroup
	var mu sync.Mutex
	toolResponses = make([]map[string]interface{}, len(toolCalls))

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc provider.ToolCall) {
			defer wg.Done()
			toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			result, err := al.toolRegistry.Execute(toolCtx, tc.Name, json.RawMessage(tc.Arguments))

			if err != nil {
				logger.Warn("Tool execution failed", logger.String("tool", tc.Name), logger.ErrorField(err))
				if trace != nil {
					trace.AddToolCall(tc.Name, tc.ID, true, 0)
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
					trace.AddToolCall(tc.Name, tc.ID, false, 0)
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
