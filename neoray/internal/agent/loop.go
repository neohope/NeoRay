package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

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

// 代理循环常量
const (
	defaultMaxIterations     = 10
	defaultPendingQueueSize  = 20
	defaultStreamChanSize    = 100
	goroutineTimeout         = 10 * time.Minute
	compactThresholdMessages = 100
	toolExecutionTimeout     = 30 * time.Second
	maxConcurrentToolCalls   = 8 // 工具调用并发上限
)

// StateTraceEntry 状态追踪记录
type StateTraceEntry struct {
	State     TurnState
	StartedAt time.Time
	Duration  time.Duration
	Event     StateEvent
	Error     error
}

// providerSnapshotKey 是存储 ProviderSnapshot 的 context key
type providerSnapshotKey struct{}

// ProviderSnapshot 提供者配置快照，用于在一次回合中保持一致的 provider 视图
type ProviderSnapshot struct {
	ProviderName   string
	Model          string
	Settings       provider.GenerationSettings
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

	// 工具集成（从 Agent 迁移）
	runtimeState       *tools.RuntimeState
	goalManager        *session.GoalManager
	fileStateStore     *tools.FileStateStore
	execSessionManager *tools.ExecSessionManager
	messageTool        *tools.MessageTool

	// 状态转换表
	transitions map[TurnState]map[StateEvent]TurnState

	// 并发控制
	running       bool
	sessionLocks  map[string]*sessionLockEntry
	pendingQueues   map[string]chan *bus.InboundMessage
	concurrencyGate chan struct{}

	// 工具绑定锁 — 保护 wireToolsForSession + 工具执行的原子性
	toolWireMu sync.Mutex

	// P1-14: 全局取消 context，Stop() 时取消所有后台 goroutine
	globalCtx    context.Context
	globalCancel context.CancelFunc

	mu             sync.RWMutex
	currentSession *session.Session
}

// sessionLockEntry 引用计数的会话锁
type sessionLockEntry struct {
	mu  sync.Mutex
	ref int
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
	globalCtx, globalCancel := context.WithCancel(context.Background())
	al := &AgentLoop{
		cfg:           cfg,
		providerMgr:   providerMgr,
		sessionMgr:    sessionMgr,
		toolRegistry:  toolRegistry,
		sessionLocks:  make(map[string]*sessionLockEntry),
		pendingQueues: make(map[string]chan *bus.InboundMessage),
		globalCtx:     globalCtx,
		globalCancel:  globalCancel,
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

	// 初始化运行时状态和工具（从 Agent 迁移）
	al.runtimeState = tools.NewRuntimeState()
	al.goalManager = session.NewGoalManager(al.sessionMgr, al.msgBus)
	al.fileStateStore = tools.NewFileStateStore()
	al.execSessionManager = tools.NewExecSessionManager()

	// 注册额外工具
	if al.toolRegistry != nil {
		al.toolRegistry.Register(tools.NewWriteStdinToolWithSessionManager(al.cfg, al.execSessionManager))
		al.toolRegistry.Register(tools.NewListExecSessionsToolWithSessionManager(al.cfg, al.execSessionManager))
		al.messageTool = tools.NewMessageToolWithBus(al.cfg, al.msgBus)
		al.toolRegistry.Register(al.messageTool)
		al.toolRegistry.Register(tools.NewReflectionTool(al.runtimeState, true))
		al.toolRegistry.Register(tools.NewLongTaskTool(al.goalManager, func() *session.Session {
			al.mu.RLock()
			defer al.mu.RUnlock()
			return al.currentSession
		}))
		al.toolRegistry.Register(tools.NewCompleteGoalTool(al.goalManager, func() *session.Session {
			al.mu.RLock()
			defer al.mu.RUnlock()
			return al.currentSession
		}))
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
	// P1-14: 取消所有后台 goroutine
	if al.globalCancel != nil {
		al.globalCancel()
	}
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
	lock.mu.Lock()
	defer lock.mu.Unlock()
	defer al.releaseSessionLock(sessionKey)

	result, err := al.processMessage(ctx, msg, sessionKey)

	// 清理追踪会话，防止内存泄漏
	if al.traceManager != nil {
		al.traceManager.RemoveSession(sessionKey)
	}

	return result, err
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
			lock.mu.Lock()
			err := al.dispatchCommandInline(ctx, msg, sessionKey, raw)
			lock.mu.Unlock()
			al.releaseSessionLock(sessionKey)
			return err
		}
	}

	// 启动新任务处理 — 派生独立 context，避免捕获短生命周期的请求 context
	// 锁顺序约定：始终先获取 al.mu 再获取 lock.mu，防止死锁
	go func() {
		lock.mu.Lock()
		lockAcquiredAt := time.Now()
		defer func() {
			lockHeld := time.Since(lockAcquiredAt)
			if lockHeld > 30*time.Second {
				logger.Warn("Session lock held for extended duration",
					logger.String("session_key", sessionKey),
					logger.String("duration", lockHeld.String()),
				)
			}
			lock.mu.Unlock()
			al.releaseSessionLock(sessionKey)
		}()

		// 创建 pending 队列（al.mu 已在 getSessionLock 中释放，此处重新获取）
		pendingQueue := make(chan *bus.InboundMessage, defaultPendingQueueSize)
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

		// P1-14: 使用全局 context，Stop() 时可取消所有后台 goroutine。
		// 保留 logger/provider 值，但使用全局取消信号。
		goroutineCtx := al.globalCtx
		var cancel context.CancelFunc
		goroutineCtx, cancel = context.WithTimeout(goroutineCtx, goroutineTimeout)
		defer cancel()

		_, err := al.processMessage(goroutineCtx, msg, sessionKey)
		if err != nil {
			logger.Error("Error processing message", logger.ErrorField(err))
		}

		// 清理追踪会话，防止内存泄漏
		if al.traceManager != nil {
			al.traceManager.RemoveSession(sessionKey)
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

	// finishChat: Hook + 内存记录（确保 bus 路径也能记录）
	if turnCtx.Session != nil && turnCtx.FinalContent != "" {
		al.finishChat(ctx, turnCtx.Session, turnCtx.Msg.Content, &ChatResult{
			Message: &session.Message{Role: "assistant", Content: turnCtx.FinalContent},
		})
	}

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

	// 恢复运行时检查点：从 session metadata 恢复 goal state 等关键状态
	if turnCtx.Session != nil {
		if goalRaw, ok := turnCtx.Session.Metadata[session.GoalStateKey]; ok {
			logger.Debug("Restored goal state from session metadata",
				logger.String("session_id", turnCtx.Session.ID))
			_ = goalRaw // GoalState 由 GoalManager 管理，此处仅做存在性检查
		}
	}

	// 恢复待处理用户回合：pendingQueues 是内存结构，进程重启后不持久化
	// 如果会话有待处理消息，它们会在 handleInboundMessage 中被正常排队处理

	// 文档提取：session 摘要由 ContextBuilder.BuildMessages 在 stateBuild 阶段自动处理

	return StateEventOK, nil
}

// stateCompact 压缩会话历史
func (al *AgentLoop) stateCompact(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Compact", logger.String("session_key", turnCtx.SessionKey))

	// 如果消息数量超过阈值，触发后台压缩
	if al.memoryManager != nil && al.memoryManager.Store() != nil && len(turnCtx.Session.Messages) > compactThresholdMessages {
		go func() {
			if err := al.memoryManager.Store().CompactHistory(); err != nil {
				logger.Warn("Background compaction failed", logger.ErrorField(err))
			}
		}()
	}

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

	// 持久化用户消息（系统消息使用 system 角色）
	if !turnCtx.UserPersistedEarly {
		var sessMsg session.Message
		if turnCtx.Msg.Type == bus.MessageTypeSystem {
			sessMsg = session.NewSystemMessage("", "", "", turnCtx.Msg.Content)
		} else {
			sessMsg = session.NewUserMessage("", "", "", turnCtx.Msg.Content)
		}
		turnCtx.Session.AddMessage(sessMsg)
		turnCtx.UserPersistedEarly = true
	}

	// token 预算压缩：在发送到 LLM 前压缩过长的会话历史
	if al.memoryManager != nil {
		if err := al.memoryManager.MaybeConsolidateSession(ctx, turnCtx.Session); err != nil {
			logger.Warn("Session consolidation failed", logger.ErrorField(err))
		}
	}

	return StateEventOK, nil
}

// wireToolsForSession 将 FileStateStore 和 ExecSessionManager 绑定到当前会话。
// 使用 toolWireMu 保证绑定的原子性，防止不同 session 并发修改共享工具状态。
func (al *AgentLoop) wireToolsForSession(sessionID string) {
	al.toolWireMu.Lock()
	defer al.toolWireMu.Unlock()
	al.wireToolsForSessionLocked(sessionID)
}

// wireToolsForSessionLocked 内部方法，调用者必须持有 toolWireMu。
func (al *AgentLoop) wireToolsForSessionLocked(sessionID string) {
	if al.fileStateStore != nil {
		fileStates := al.fileStateStore.ForSession(sessionID)
		if tool, ok := al.toolRegistry.Get("filesystem"); ok {
			if fsTool, ok := tool.(*tools.FileSystemTool); ok {
				fsTool.SetFileStates(fileStates)
			}
		}
		if tool, ok := al.toolRegistry.Get("apply_patch"); ok {
			if patchTool, ok := tool.(*tools.ApplyPatchTool); ok {
				patchTool.SetFileStates(fileStates)
			}
		}
	}
	if al.execSessionManager != nil {
		if tool, ok := al.toolRegistry.Get("shell"); ok {
			if shellTool, ok := tool.(*tools.ShellTool); ok {
				shellTool.SetSessionManager(al.execSessionManager)
				shellTool.SetSessionKey(sessionID)
			}
		}
		if tool, ok := al.toolRegistry.Get("write_stdin"); ok {
			if writeTool, ok := tool.(*tools.WriteStdinTool); ok {
				writeTool.SetSessionKey(sessionID)
			}
		}
		if tool, ok := al.toolRegistry.Get("list_exec_sessions"); ok {
			if listTool, ok := tool.(*tools.ListExecSessionsTool); ok {
				listTool.SetSessionKey(sessionID)
			}
		}
	}
}

// wireAndExecuteToolCalls 原子地绑定工具到 session 并执行工具调用。
// 持有 toolWireMu 贯穿整个绑定+执行过程，防止并发 session 的状态串扰。
func (al *AgentLoop) wireAndExecuteToolCalls(
	ctx context.Context,
	sessionID string,
	toolCalls []provider.ToolCall,
	trace *TraceSession,
) []map[string]interface{} {
	al.toolWireMu.Lock()
	defer al.toolWireMu.Unlock()

	al.wireToolsForSessionLocked(sessionID)
	return al.executeToolCallsLocked(ctx, toolCalls, trace)
}

// stateRun 运行 LLM
func (al *AgentLoop) stateRun(ctx context.Context, turnCtx *TurnContext) (StateEvent, error) {
	logger.Debug("State: Run", logger.String("session_key", turnCtx.SessionKey))

	// Hook: BeforeIter
	if err := al.hook.BeforeIter(ctx, turnCtx.Session); err != nil {
		logger.Warn("Hook BeforeIter failed", logger.ErrorField(err))
	}

	if turnCtx.VisibleRunStartedAt == nil {
		now := time.Now()
		turnCtx.VisibleRunStartedAt = &now
	}

	// 设置当前会话
	al.mu.Lock()
	al.currentSession = turnCtx.Session
	al.mu.Unlock()

	// 创建 Trace
	var trace *TraceSession
	if al.traceManager.IsEnabled() {
		trace = al.traceManager.GetOrCreateSession(turnCtx.Session.ID)
	}

	p, err := al.providerMgr.GetProvider(al.cfg.LLM.DefaultProvider)
	if err != nil || p == nil {
		p = al.providerMgr.DefaultProvider()
	}

	if p == nil {
		errMsg := "⚠️ No LLM provider configured! Please edit your neoray.toml and add an API key for Anthropic or OpenAI."
		logger.Error("No LLM provider available", logger.String("default_provider", al.cfg.LLM.DefaultProvider))
		if trace != nil {
			trace.AddError(fmt.Errorf("no provider"), "Provider resolution")
		}
		turnCtx.FinalContent = errMsg
		return StateEventOK, nil
	}

	var providerTools []provider.Tool
	if al.toolRegistry != nil {
		for _, def := range al.toolRegistry.GetDefinitions() {
			var schema map[string]interface{}
			if err := json.Unmarshal(def.InputSchema, &schema); err != nil {
				logger.Warn("Skipping tool with invalid InputSchema",
					logger.String("tool", def.Name), logger.ErrorField(err))
				continue
			}
			providerTools = append(providerTools, provider.Tool{
				Name:        def.Name,
				Description: def.Description,
				InputSchema: schema,
			})
		}
	}

	maxIterations := defaultMaxIterations
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

		logger.Info("Calling LLM",
			logger.String("session_key", turnCtx.SessionKey),
			logger.String("provider", p.Name()),
			logger.Int("iteration", iterations+1))

		llmStart := time.Now()
		resp, err := al.callLLMWithRetry(ctx, p, req)
		llmDuration := time.Since(llmStart)

		logger.Info("LLM call completed",
			logger.String("session_key", turnCtx.SessionKey),
			logger.Duration("duration", llmDuration),
			logger.Bool("error", err != nil),
			logger.Bool("response_nil", resp == nil))

		if trace != nil && resp != nil && resp.Usage != nil {
			trace.AddLLMCall(iterations+1, resp.Usage.InputTokens, resp.Usage.OutputTokens, llmDuration)
		}

		if err != nil {
			logger.Error("LLM call failed after retries", logger.ErrorField(err), logger.Int("iteration", iterations+1))
			if trace != nil {
				trace.AddError(err, fmt.Sprintf("LLM call iteration %d", iterations+1))
			}
			errMsg := "I'm having trouble connecting to the AI service right now. Please try again later."
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

		// 有工具调用，继续执行（原子绑定+执行，防止并发 session 状态串扰）
		totalToolCalls += len(resp.ToolCalls)
		toolResponses := al.wireAndExecuteToolCalls(ctx, turnCtx.Session.ID, resp.ToolCalls, trace)
		toolRespJSON, err := json.Marshal(toolResponses)
		if err != nil {
			logger.Warn("Failed to marshal tool responses", logger.ErrorField(err))
			toolRespJSON = []byte("{}")
		}
		toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
		turnCtx.Session.AddMessage(toolMsg)
	}

	logger.Warn("Max tool iterations reached")
	if trace != nil {
		trace.AddInfo("Max tool iterations reached", map[string]interface{}{"max_iterations": maxIterations})
	}

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

	// 后台触发空闲会话压缩检查
	if al.memoryManager != nil {
		go al.memoryManager.CheckExpiredSessions(ctx)
	}

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

// dispatchCommandInline 内联分发命令（调用者必须已持有 sessionLock）
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

// getSessionLock 获取会话锁（调用者必须在使用后调用 releaseSessionLock）
func (al *AgentLoop) getSessionLock(sessionKey string) *sessionLockEntry {
	al.mu.Lock()
	defer al.mu.Unlock()

	entry, ok := al.sessionLocks[sessionKey]
	if !ok {
		entry = &sessionLockEntry{}
		al.sessionLocks[sessionKey] = entry
	}
	entry.ref++
	return entry
}

// releaseSessionLock 释放会话锁引用，引用归零时从 map 中移除
func (al *AgentLoop) releaseSessionLock(sessionKey string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	if entry, ok := al.sessionLocks[sessionKey]; ok {
		entry.ref--
		if entry.ref <= 0 {
			delete(al.sessionLocks, sessionKey)
		}
	}
}

// Chat 发送聊天消息（同步入口，替代 Agent.Chat）
func (al *AgentLoop) Chat(ctx context.Context, sess *session.Session, userInput string) (*ChatResult, error) {
	startTime := time.Now()

	// Hook: BeforeIter
	if err := al.hook.BeforeIter(ctx, sess); err != nil {
		logger.Warn("Hook BeforeIter failed", logger.ErrorField(err))
	}

	// 跟踪活跃会话
	if al.memoryManager != nil {
		al.memoryManager.TrackSession(sess.ID)
		defer al.memoryManager.UntrackSession(sess.ID)
	}

	// 从会话中提取 ChannelID 和 ChatID
	// 会话 ID 格式: <channel>:<chat_id> 或 <channel>:<chat_id>:<topic_id>
	channelID := sess.ChannelID
	chatID := sess.ID // 使用完整会话 ID 作为 ChatID，feishu 频道会正确处理

	logger.Debug("Chat: extracted channel info", logger.String("sess_id", sess.ID), logger.String("channel_id", channelID), logger.String("chat_id", chatID))

	// 构建 bus 消息
	msg := &bus.InboundMessage{
		ID:        fmt.Sprintf("chat:%d", time.Now().UnixNano()),
		ChannelID: channelID,
		ChatID:    chatID,
		UserID:    "direct",
		Content:   userInput,
		Metadata:  make(map[string]interface{}),
	}

	sessionKey := sess.ID
	lock := al.getSessionLock(sessionKey)
	lock.mu.Lock()
	defer lock.mu.Unlock()
	defer al.releaseSessionLock(sessionKey)

	// 创建 Trace
	trace := al.traceManager.GetOrCreateSession(sess.ID)
	if trace != nil {
		trace.AddInfo(fmt.Sprintf("Chat started: %s", sess.ID), nil)
	}
	// 清理追踪会话，防止内存泄漏（trace 对象仍被 result 持有）
	defer al.traceManager.RemoveSession(sess.ID)

	// 通过状态机处理
	outbound, err := al.processMessage(ctx, msg, sessionKey)
	if err != nil {
		// 当 processMessage 失败时，添加错误消息到 session 并返回
		errMsg := "抱歉，处理您的消息时出现问题，请稍后重试。"
		assistantMsg := session.NewAssistantMessage("", "", "", errMsg)
		sess.AddMessage(assistantMsg)

		// 保存 session 以记录错误消息
		if saveErr := al.sessionMgr.SaveSession(sess); saveErr != nil {
			logger.Warn("Failed to save session after error", logger.ErrorField(saveErr))
		}

		return &ChatResult{
			Message:    &assistantMsg,
			Error:      err,
			TokenUsage: al.tokenManager.GetSessionUsage(sess.ID),
			Iterations: 1,
			Duration:   time.Since(startTime),
			Trace:      trace,
		}, nil
	}

	// 提取最终内容
	finalContent := ""
	if outbound != nil {
		finalContent = outbound.Content
	}

	// 查找最后的 assistant 消息
	var lastMsg *session.Message
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		if sess.Messages[i].Role == "assistant" {
			lastMsg = &sess.Messages[i]
			break
		}
	}
	if lastMsg == nil && finalContent != "" {
		msg := session.NewAssistantMessage("", "", "", finalContent)
		lastMsg = &msg
	}

	// 如果仍然没有 assistant 消息，创建一个默认消息
	if lastMsg == nil {
		defaultMsg := session.NewAssistantMessage("", "", "", "抱歉，我无法处理您的消息，请稍后重试。")
		sess.AddMessage(defaultMsg)
		lastMsg = &defaultMsg
		logger.Warn("No assistant message found, created default message",
			logger.String("session_id", sess.ID))
	}

	result := &ChatResult{
		Message:    lastMsg,
		TokenUsage: al.tokenManager.GetSessionUsage(sess.ID),
		Trace:      trace,
		Duration:   time.Since(startTime),
	}

	// finishChat: Hook + 内存记录
	al.finishChat(ctx, sess, userInput, result)
	return result, nil
}

// finishChat 完成聊天，调用 AfterIter Hook + 内存记录
func (al *AgentLoop) finishChat(ctx context.Context, sess *session.Session, userInput string, result *ChatResult) {
	if err := al.hook.AfterIter(ctx, sess, result); err != nil {
		logger.Warn("Hook AfterIter failed", logger.ErrorField(err))
	}

	// 记录对话到记忆系统
	if al.memoryManager != nil && result != nil && result.Message != nil {
		if userInput != "" && result.Message.Content != "" {
			if _, err := al.memoryManager.AppendHistory(userInput, result.Message.Content); err != nil {
				logger.Warn("Failed to append to history", logger.ErrorField(err))
			}
		}
	}
}

// ChatStream 发送聊天消息（流式入口，替代 Agent.ChatStream）
func (al *AgentLoop) ChatStream(ctx context.Context, sess *session.Session, userInput string) (<-chan StreamChunk, error) {
	resultChan := make(chan StreamChunk, defaultStreamChanSize)

	// 获取 session 锁，保证整个流式处理期间 session 状态的一致性
	sessionKey := sess.ID
	lock := al.getSessionLock(sessionKey)
	lock.mu.Lock()

	// 设置当前会话
	al.mu.Lock()
	al.currentSession = sess
	al.mu.Unlock()

	// 先检查是否是指令
	if al.cmdManager != nil {
		if resp, isCmd, err := al.cmdManager.Process(ctx, sess, userInput); isCmd {
			go func() {
				defer lock.mu.Unlock()
				defer al.releaseSessionLock(sessionKey)
				defer close(resultChan)
				var assistantMsg session.Message
				if err != nil {
					assistantMsg = session.NewAssistantMessage("", "", "", fmt.Sprintf("❌ Command error: %v", err))
				} else {
					assistantMsg = session.NewAssistantMessage("", "", "", resp)
				}
				sess.AddMessage(assistantMsg)
				if saveErr := al.sessionMgr.SaveSession(sess); saveErr != nil {
					logger.Error("Failed to save session", logger.ErrorField(saveErr))
				}
				sendChunk(ctx, resultChan, StreamChunk{Type: "end", Content: resp, SessionMsg: &assistantMsg})
			}()
			return resultChan, nil
		}
	}

	// 跟踪活跃会话
	if al.memoryManager != nil {
		al.memoryManager.TrackSession(sess.ID)
	}

	// 设置 spawn 工具上下文
	if al.spawnTool != nil {
		al.spawnTool.SetOriginContext("cli", "direct", sess.ID, "")
	}

	userMsg := session.NewUserMessage("", "", "", userInput)
	sess.AddMessage(userMsg)

	if err := al.hook.BeforeStream(ctx, sess); err != nil {
		logger.Warn("Hook BeforeStream failed", logger.ErrorField(err))
	}

	go func() {
		defer close(resultChan)
		defer lock.mu.Unlock()
		defer al.releaseSessionLock(sessionKey)
		// 清理活跃会话跟踪，防止内存泄漏
		if al.memoryManager != nil {
			defer al.memoryManager.UntrackSession(sess.ID)
		}

		var trace *TraceSession
		if al.traceManager.IsEnabled() {
			trace = al.traceManager.GetOrCreateSession(sess.ID)
			// 清理追踪会话，防止内存泄漏
			defer al.traceManager.RemoveSession(sess.ID)
		}

		p, err := al.providerMgr.GetProvider(al.cfg.LLM.DefaultProvider)
		if err != nil || p == nil {
			p = al.providerMgr.DefaultProvider()
		}

		if p == nil {
			sendChunk(ctx, resultChan, StreamChunk{Type: "error", Error: fmt.Errorf("no LLM provider configured")})
			return
		}

		var providerTools []provider.Tool
		if al.toolRegistry != nil {
			for _, def := range al.toolRegistry.GetDefinitions() {
				var schema map[string]interface{}
				if err := json.Unmarshal(def.InputSchema, &schema); err != nil {
					logger.Warn("Skipping tool with invalid InputSchema",
						logger.String("tool", def.Name), logger.ErrorField(err))
					continue
				}
				providerTools = append(providerTools, provider.Tool{Name: def.Name, Description: def.Description, InputSchema: schema})
			}
		}

		maxIterations := al.cfg.Agent.MaxIterations
		if maxIterations <= 0 {
			maxIterations = defaultMaxIterations
		}

		for iteration := 0; iteration < maxIterations; iteration++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			msgs := al.contextBuilder.BuildMessages(sess)
			req := &provider.ChatRequest{
				Messages: msgs, Tools: providerTools,
			}

			if streamProvider, ok := p.(provider.StreamToolProvider); ok {
				done, err := al.handleNativeStreamTool(ctx, streamProvider, req, sess, resultChan, trace, providerTools)
				if err != nil {
					sendChunk(ctx, resultChan, StreamChunk{Type: "error", Error: err})
					return
				}
				if done {
					if err := al.hook.AfterStream(ctx, sess); err != nil {
						logger.Warn("Hook AfterStream failed", logger.ErrorField(err))
					}
					return
				}
			} else {
				done, err := al.handleFallbackStream(ctx, p, req, sess, resultChan, trace, providerTools)
				if err != nil {
					sendChunk(ctx, resultChan, StreamChunk{Type: "error", Error: err})
					return
				}
				if done {
					if err := al.hook.AfterStream(ctx, sess); err != nil {
						logger.Warn("Hook AfterStream failed", logger.ErrorField(err))
					}
					return
				}
			}
		}

		logger.Warn("Max tool iterations reached in stream")
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
		sess.AddMessage(assistantMsg)
		sendChunk(ctx, resultChan, StreamChunk{Type: "text", Content: maxIterMsg})
		sendChunk(ctx, resultChan, StreamChunk{Type: "end", Content: maxIterMsg, SessionMsg: &assistantMsg})
		if err := al.hook.AfterStream(ctx, sess); err != nil {
			logger.Warn("Hook AfterStream failed", logger.ErrorField(err))
		}
	}()

	return resultChan, nil
}

// handleNativeStreamTool 处理原生流式工具调用
func (al *AgentLoop) handleNativeStreamTool(
	ctx context.Context,
	p provider.StreamToolProvider,
	req *provider.ChatRequest,
	sess *session.Session,
	resultChan chan<- StreamChunk,
	trace *TraceSession,
	providerTools []provider.Tool,
) (bool, error) {
	stream, err := p.ChatStreamWithTools(ctx, req)
	if err != nil {
		return false, err
	}

	var fullContent strings.Builder
	// 按 Index 累积流式工具调用 delta，支持多 chunk 传输 arguments
	toolCallMap := make(map[int]*provider.ToolCall)
	var toolCallStarted bool
	var finishReason string

	for chunk := range stream {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		default:
		}

		if chunk.Error != nil {
			return false, chunk.Error
		}

		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			if !sendChunk(ctx, resultChan, StreamChunk{Type: "text", Content: chunk.Content}) {
				return true, ctx.Err()
			}
			if err := al.hook.OnStreamDelta(ctx, chunk.Content); err != nil {
				logger.Warn("Hook OnStreamDelta failed", logger.ErrorField(err))
			}
		}

		// 累积所有工具调用 delta（按 index 合并 id/name/arguments）
		if len(chunk.ToolCalls) > 0 {
			if !toolCallStarted {
				toolCallStarted = true
				if !sendChunk(ctx, resultChan, StreamChunk{Type: "tool_start", ToolCalls: chunk.ToolCalls}) {
					return true, ctx.Err()
				}
			}
			for _, tc := range chunk.ToolCalls {
				// 使用 tc.ID 作为 index 的备选（当 Index 未设置时用 ID 区分）
				key := len(toolCallMap) // 默认按追加顺序
				if tc.ID != "" {
					// 检查是否已有同 ID 的条目
					for k, existing := range toolCallMap {
						if existing.ID == tc.ID {
							key = k
							break
						}
					}
				}
				existing, ok := toolCallMap[key]
				if !ok {
					tcCopy := tc
					toolCallMap[key] = &tcCopy
				} else {
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Name != "" {
						existing.Name = tc.Name
					}
					if tc.Arguments != "" {
						existing.Arguments += tc.Arguments
					}
				}
			}
		}

		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
			// 继续处理当前 chunk 中可能存在的剩余 choice 数据
			// 不立即 break，等循环自然结束
		}
	}

	// 从 map 中提取完整的工具调用列表
	var currentToolCalls []provider.ToolCall
	for _, tc := range toolCallMap {
		currentToolCalls = append(currentToolCalls, *tc)
	}

	if len(currentToolCalls) > 0 {
		assistantMsg := session.NewAssistantMessage("", "", "", fullContent.String())
		assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(currentToolCalls))
		for _, tc := range currentToolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		sess.AddMessage(assistantMsg)

		toolResponses := al.wireAndExecuteToolCalls(ctx, sess.ID, currentToolCalls, trace)
		if !sendChunk(ctx, resultChan, StreamChunk{Type: "tool_result", ToolResults: toolResponses}) {
			return true, ctx.Err()
		}

		toolRespJSON, marshalErr := json.Marshal(toolResponses)
		if marshalErr != nil {
			logger.Warn("Failed to marshal tool responses", logger.ErrorField(marshalErr))
			toolRespJSON = []byte("{}")
		}
		toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
		sess.AddMessage(toolMsg)
		return false, nil
	}

	if fullContent.Len() > 0 {
		assistantMsg := session.NewAssistantMessage("", "", "", fullContent.String())
		sess.AddMessage(assistantMsg)

		if al.continuationMgr.IsTruncated(finishReason) {
			al.streamContinuation(ctx, p, sess, providerTools, resultChan)
			if len(sess.Messages) > 0 {
				assistantMsg = sess.Messages[len(sess.Messages)-1]
			}
		}

		if err := al.sessionMgr.SaveSession(sess); err != nil {
			logger.Warn("Failed to save session", logger.ErrorField(err))
		}
		sendChunk(ctx, resultChan, StreamChunk{Type: "end", Content: assistantMsg.Content, SessionMsg: &assistantMsg})
	}
	return true, nil
}

// handleFallbackStream 处理回退流式（非流式 provider 模拟流式）
func (al *AgentLoop) handleFallbackStream(
	ctx context.Context,
	p provider.Provider,
	req *provider.ChatRequest,
	sess *session.Session,
	resultChan chan<- StreamChunk,
	trace *TraceSession,
	providerTools []provider.Tool,
) (bool, error) {
	resp, err := al.callLLMWithRetry(ctx, p, req)
	if err != nil {
		return false, err
	}

	if resp.Content != "" {
		const chunkSize = 64
		runes := []rune(resp.Content)
		for i := 0; i < len(runes); i += chunkSize {
			select {
			case <-ctx.Done():
				return true, ctx.Err()
			default:
			}
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			chunk := string(runes[i:end])
			if !sendChunk(ctx, resultChan, StreamChunk{Type: "text", Content: chunk}) {
				return true, ctx.Err()
			}
			if err := al.hook.OnStreamDelta(ctx, chunk); err != nil {
				logger.Warn("Hook OnStreamDelta failed", logger.ErrorField(err))
			}
		}
	}

	if len(resp.ToolCalls) > 0 && al.toolRegistry != nil {
		if !sendChunk(ctx, resultChan, StreamChunk{Type: "tool_start", ToolCalls: resp.ToolCalls}) {
			return true, ctx.Err()
		}

		assistantMsg := session.NewAssistantMessage("", "", "", resp.Content)
		assistantMsg.ToolCalls = make([]session.ToolCall, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		sess.AddMessage(assistantMsg)

		toolResponses := al.wireAndExecuteToolCalls(ctx, sess.ID, resp.ToolCalls, trace)
		if !sendChunk(ctx, resultChan, StreamChunk{Type: "tool_result", ToolResults: toolResponses}) {
			return true, ctx.Err()
		}

		toolRespJSON, marshalErr := json.Marshal(toolResponses)
		if marshalErr != nil {
			logger.Warn("Failed to marshal tool responses", logger.ErrorField(marshalErr))
			toolRespJSON = []byte("{}")
		}
		toolMsg := session.NewToolMessage("", "", "", string(toolRespJSON))
		sess.AddMessage(toolMsg)
		return false, nil
	}

	if resp.Content != "" {
		assistantMsg := session.NewAssistantMessage("", "", "", resp.Content)
		sess.AddMessage(assistantMsg)

		if al.continuationMgr.IsTruncated(resp.FinishReason) {
			al.streamContinuation(ctx, p, sess, providerTools, resultChan)
			assistantMsg = sess.Messages[len(sess.Messages)-1]
		}

		if err := al.sessionMgr.SaveSession(sess); err != nil {
			logger.Warn("Failed to save session", logger.ErrorField(err))
		}
		sendChunk(ctx, resultChan, StreamChunk{Type: "end", Content: assistantMsg.Content, SessionMsg: &assistantMsg})
	}
	return true, nil
}

// streamContinuation 处理流式续轮
func (al *AgentLoop) streamContinuation(
	ctx context.Context,
	p provider.Provider,
	sess *session.Session,
	providerTools []provider.Tool,
	resultChan chan<- StreamChunk,
) {
	var continuationCount int
	var stillTruncated bool
	for continuationCount = 0; al.continuationMgr.ShouldContinue(continuationCount); continuationCount++ {
		contResp, truncated, contErr := al.continuationMgr.ExecuteContinuation(
			ctx, p, sess, providerTools, al.callLLMWithRetry)
		if contErr != nil {
			logger.Warn("Stream continuation failed, stopping", logger.ErrorField(contErr))
			break
		}

		const chunkSize = 64
		contentRunes := []rune(contResp.Content)
		for i := 0; i < len(contentRunes); i += chunkSize {
			select {
			case <-ctx.Done():
				return
			default:
			}
			end := i + chunkSize
			if end > len(contentRunes) {
				end = len(contentRunes)
			}
			chunk := string(contentRunes[i:end])
			if !sendChunk(ctx, resultChan, StreamChunk{Type: "text", Content: chunk}) {
				return
			}
			if err := al.hook.OnStreamDelta(ctx, chunk); err != nil {
				logger.Warn("Hook OnStreamDelta failed", logger.ErrorField(err))
			}
		}

		al.continuationMgr.MergeContinuation(sess, contResp.Content)
		stillTruncated = truncated

		if !stillTruncated {
			break
		}
	}
	if stillTruncated {
		logger.Warn("Still truncated after max continuations in stream",
			logger.Int("continuations", continuationCount))
	}
}

// refreshProviderSnapshot 刷新提供者快照，在回合开始时捕获 provider 配置
func (al *AgentLoop) refreshProviderSnapshot(ctx context.Context) context.Context {
	p, err := al.providerMgr.GetProvider(al.cfg.LLM.DefaultProvider)
	if err != nil || p == nil {
		p = al.providerMgr.DefaultProvider()
	}
	if p == nil {
		return ctx
	}

	snapshot := ProviderSnapshot{
		ProviderName: p.Name(),
		Model:        p.GetDefaultModel(),
		Settings:     p.GetGenerationSettings(),
	}

	// 从 per-provider 配置覆盖 settings
	if pcfg, ok := al.cfg.LLM.Providers[p.Name()]; ok {
		if pcfg.MaxTokens > 0 {
			snapshot.Settings.MaxTokens = pcfg.MaxTokens
		}
		if pcfg.Temperature > 0 {
			snapshot.Settings.Temperature = pcfg.Temperature
		}
		if pcfg.ReasoningEffort != "" {
			snapshot.Settings.ReasoningEffort = pcfg.ReasoningEffort
		}
	}

	return context.WithValue(ctx, providerSnapshotKey{}, snapshot)
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

	logger.Info("Calling LLM",
		logger.String("provider", p.Name()),
		logger.Int("messages", len(req.Messages)),
		logger.Int("tools", len(req.Tools)))

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := p.Chat(ctx, req)

		if err != nil {
			logger.Warn("LLM call failed",
				logger.Int("attempt", attempt+1),
				logger.ErrorField(err))
		} else {
			logger.Info("LLM response received",
				logger.Int("content_length", len(resp.Content)),
				logger.Int("tool_calls", len(resp.ToolCalls)),
				logger.String("finish_reason", resp.FinishReason))
		}

		if err == nil {
			return resp, nil
		}
		lastErr = err

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

// executeToolCallsLocked 执行工具调用（带并发限制和 panic 恢复）。
// 调用者必须持有 toolWireMu。外部调用请使用 wireAndExecuteToolCalls。
func (al *AgentLoop) executeToolCallsLocked(
	ctx context.Context,
	toolCalls []provider.ToolCall,
	trace *TraceSession,
) []map[string]interface{} {
	var wg sync.WaitGroup
	var mu sync.Mutex
	toolResponses := make([]map[string]interface{}, len(toolCalls))
	sem := semaphore.NewWeighted(maxConcurrentToolCalls)

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc provider.ToolCall) {
			defer wg.Done()
			// P1-13: panic 恢复 — 单个工具 panic 不应终止整个 agent
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Tool execution panicked",
						logger.String("tool", tc.Name),
						logger.String("panic", fmt.Sprintf("%v", r)))
					mu.Lock()
					toolResponses[idx] = map[string]interface{}{
						"tool_use_id": tc.ID,
						"content":     fmt.Sprintf("Tool panicked: %v", r),
						"is_error":    true,
					}
					mu.Unlock()
				}
			}()

			// 获取并发许可，限制同时运行的工具调用数量
			if err := sem.Acquire(ctx, 1); err != nil {
				logger.Warn("Failed to acquire semaphore for tool call", logger.ErrorField(err))
				mu.Lock()
				toolResponses[idx] = map[string]interface{}{
					"tool_use_id": tc.ID,
					"content":     fmt.Sprintf("Error: %v", err),
					"is_error":    true,
				}
				mu.Unlock()
				return
			}
			defer sem.Release(1)

			toolCtx, cancel := context.WithTimeout(ctx, toolExecutionTimeout)
			defer cancel()

			toolStart := time.Now()
			result, err := al.toolRegistry.Execute(toolCtx, tc.Name, json.RawMessage(tc.Arguments))
			toolDuration := time.Since(toolStart)

			if err != nil {
				logger.Warn("Tool execution failed", logger.String("tool", tc.Name), logger.ErrorField(err))
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
