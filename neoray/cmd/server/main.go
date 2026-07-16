package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"neoray/internal/agent"
	"neoray/internal/api"
	"neoray/internal/bus"
	"neoray/internal/channel"
	"neoray/internal/config"
	"neoray/internal/cron"
	"neoray/internal/logger"
	"neoray/internal/memory"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/tools"
	"neoray/internal/tui"
)

// ========== Cron 调度器适配器 ==========
// cronSchedulerAdapter 包装 *cron.CronScheduler 并实现 tools.CronSchedulerInterface

type cronSchedulerAdapter struct {
	scheduler *cron.CronScheduler
}

func (a *cronSchedulerAdapter) AddJob(name string, scheduleAny any, message string, deliver bool, channel string, to string, deleteAfterRun bool, metadata map[string]any, sessionKey string) (any, error) {
	// 将 any/map 转换为 cron.CronSchedule
	schedule, err := parseSchedule(scheduleAny)
	if err != nil {
		return nil, err
	}

	job, err := a.scheduler.AddJob(name, schedule, message, deliver, channel, to, deleteAfterRun, metadata, sessionKey)
	if err != nil {
		return nil, err
	}
	return jobToMap(job), nil
}

func (a *cronSchedulerAdapter) ListJobs(includeMetadata bool) []any {
	jobs := a.scheduler.ListJobs(includeMetadata)
	result := make([]any, len(jobs))
	for i, job := range jobs {
		result[i] = jobToMap(&job)
	}
	return result
}

func (a *cronSchedulerAdapter) RemoveJob(jobID string) string {
	return a.scheduler.RemoveJob(jobID)
}

func (a *cronSchedulerAdapter) GetJob(jobID string) any {
	job := a.scheduler.GetJob(jobID)
	if job == nil {
		return nil
	}
	return jobToMap(job)
}

// ========== 转换辅助函数 ==========

func parseSchedule(scheduleAny any) (cron.CronSchedule, error) {
	switch sched := scheduleAny.(type) {
	case map[string]any:
		kindStr, _ := sched["kind"].(string)
		switch cron.ScheduleKind(kindStr) {
		case cron.ScheduleKindAt:
			var atMs int64
			switch v := sched["at_ms"].(type) {
			case int64:
				atMs = v
			case float64:
				atMs = int64(v)
			case int:
				atMs = int64(v)
			}
			return cron.CronSchedule{
				Kind: cron.ScheduleKindAt,
				AtMS: atMs,
			}, nil
		case cron.ScheduleKindEvery:
			var everyMs int64
			switch v := sched["every_ms"].(type) {
			case int64:
				everyMs = v
			case float64:
				everyMs = int64(v)
			case int:
				everyMs = int64(v)
			}
			return cron.CronSchedule{
				Kind:    cron.ScheduleKindEvery,
				EveryMS: everyMs,
			}, nil
		case cron.ScheduleKindCron:
			expr, _ := sched["expr"].(string)
			timezone, _ := sched["timezone"].(string)
			return cron.CronSchedule{
				Kind:     cron.ScheduleKindCron,
				Expr:     expr,
				Timezone: timezone,
			}, nil
		}
	}
	return cron.CronSchedule{}, fmt.Errorf("invalid schedule format")
}

func jobToMap(job *cron.CronJob) map[string]any {
	if job == nil {
		return nil
	}
	return map[string]any{
		"id":               job.ID,
		"name":             job.Name,
		"enabled":          job.Enabled,
		"schedule":         scheduleToMap(job.Schedule),
		"payload":          payloadToMap(job.Payload),
		"state":            stateToMap(job.State),
		"created_at_ms":    job.CreatedAtMS,
		"updated_at_ms":    job.UpdatedAtMS,
		"delete_after_run": job.DeleteAfterRun,
	}
}

func scheduleToMap(s cron.CronSchedule) map[string]any {
	m := map[string]any{
		"kind": string(s.Kind),
	}
	switch s.Kind {
	case cron.ScheduleKindAt:
		m["at_ms"] = s.AtMS
	case cron.ScheduleKindEvery:
		m["every_ms"] = s.EveryMS
	case cron.ScheduleKindCron:
		m["expr"] = s.Expr
		if s.Timezone != "" {
			m["timezone"] = s.Timezone
		}
	}
	return m
}

func payloadToMap(p cron.CronPayload) map[string]any {
	return map[string]any{
		"kind":         string(p.Kind),
		"message":      p.Message,
		"deliver":      p.Deliver,
		"channel":      p.Channel,
		"to":           p.To,
		"channel_meta": p.ChannelMeta,
		"session_key":  p.SessionKey,
	}
}

func stateToMap(st cron.CronJobState) map[string]any {
	history := make([]any, len(st.RunHistory))
	for i, r := range st.RunHistory {
		history[i] = map[string]any{
			"run_at_ms":   r.RunAtMS,
			"status":      string(r.Status),
			"duration_ms": r.DurationMS,
			"error":       r.Error,
		}
	}
	return map[string]any{
		"last_run_at_ms": st.LastRunAtMS,
		"last_status":    string(st.LastStatus),
		"last_error":     st.LastError,
		"next_run_at_ms": st.NextRunAtMS,
		"run_history":    history,
	}
}

func main() {
	fmt.Println("NeoRay starting...")
	// 命令行参数
	configPath := flag.String("config", "", "path to config file")
	useTUI := flag.Bool("tui", false, "enable TUI mode (default: server-only mode)")
	flag.Parse()

	fmt.Printf("Command line flags:\n")
	fmt.Printf("  --config: %s\n", *configPath)
	fmt.Printf("  --tui: %v\n", *useTUI)

	fmt.Println("Loading config...")
	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Config loaded")

	fmt.Println("Initializing logger...")
	// 初始化日志
	if err := logger.Init(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	fmt.Println("Logger initialized")

	logger.Info("Starting neoray",
		logger.String("name", cfg.App.Name),
		logger.String("version", cfg.App.Version),
		logger.String("env", cfg.App.Env),
		logger.String("home_dir", cfg.HomeDir),
	)

	fmt.Println("Creating tool registry...")
	// 初始化工具注册表
	toolRegistry := tools.NewRegistry()
	if cfg.Tools.Workspace.Enabled {
		toolRegistry.Register(tools.NewFileSystemTool(cfg))
		logger.Info("Filesystem tool registered")
	}
	if cfg.Tools.Shell.Enabled {
		toolRegistry.Register(tools.NewShellTool(cfg))
		logger.Info("Shell tool registered")
	}
	// 注册新工具 - 统一使用 Enabled 检查
	if cfg.Tools.FindFiles.Enabled {
		toolRegistry.Register(tools.NewFindFilesTool())
		logger.Info("FindFiles tool registered")
	}
	if cfg.Tools.Grep.Enabled {
		toolRegistry.Register(tools.NewGrepTool())
		logger.Info("Grep tool registered")
	}
	if cfg.Tools.ApplyPatch.Enabled {
		toolRegistry.Register(tools.NewApplyPatchTool())
		logger.Info("ApplyPatch tool registered")
	}
	if cfg.Tools.WebSearch.Enabled {
		toolRegistry.Register(tools.NewWebSearchTool())
		logger.Info("WebSearch tool registered")
	}
	if cfg.Tools.WebFetch.Enabled {
		toolRegistry.Register(tools.NewWebFetchTool(cfg))
		logger.Info("WebFetch tool registered")
	}
	if cfg.Tools.SandboxStatus.Enabled {
		toolRegistry.Register(tools.NewSandboxStatusTool(cfg))
		logger.Info("SandboxStatus tool registered")
	}
	logger.Info("Tool registry initialized", logger.Int("tool_count", len(toolRegistry.List())))

	// 初始化会话存储和管理器（根据配置选择存储类型）
	sessionDir := cfg.ResolvePath("sessions")
	var sessionStore session.Store
	storageType := cfg.Session.Storage.Type

	switch storageType {
	case "memory":
		var memOpts []session.MemoryStoreOption
		if cfg.Session.Storage.MaxSessions > 0 {
			memOpts = append(memOpts, session.WithMaxSessions(cfg.Session.Storage.MaxSessions))
		}
		if cfg.Session.Storage.MaxMessagesPerSession > 0 {
			memOpts = append(memOpts, session.WithMaxMessagesPerSession(cfg.Session.Storage.MaxMessagesPerSession))
		}
		sessionStore = session.NewMemoryStore(memOpts...)
		logger.Info("Using memory session store")
	case "file", "":
		fileStore, fileErr := session.NewFileStore(sessionDir)
		if fileErr != nil {
			logger.Warn("Failed to create file store, falling back to memory store", logger.ErrorField(fileErr))
			sessionStore = session.NewMemoryStore()
		} else {
			sessionStore = fileStore
		}
	default:
		logger.Warn("Unknown session storage type, using file store", logger.String("type", storageType))
		fileStore, fileErr := session.NewFileStore(sessionDir)
		if fileErr != nil {
			logger.Warn("Failed to create file store, falling back to memory store", logger.ErrorField(fileErr))
			sessionStore = session.NewMemoryStore()
		} else {
			sessionStore = fileStore
		}
	}
	fmt.Println("Creating session manager...")
	sessionMgr := session.NewManager(cfg, sessionStore)

	fmt.Println("Initializing providers...")
	// 初始化 LLM 提供商
	providerMgr := initProviders(cfg)

	// 初始化记忆系统
	var memoryManager *memory.MemoryManager
	defaultProvider := providerMgr.DefaultProvider()
	if defaultProvider != nil {
		// 获取默认模型
		var model string
		if defaultProviderCfg, ok := cfg.LLM.Providers[cfg.LLM.DefaultProvider]; ok {
			model = defaultProviderCfg.Model
		}
		if model == "" {
			model = "gpt-3.5-turbo" // 默认模型
		}

		fmt.Println("Initializing memory system...")
		memoryManager = memory.NewMemoryManager(cfg, defaultProvider, model, sessionMgr)
		logger.Info("Memory system initialized")

		// 注册记忆工具
		memoryTool := tools.NewMemoryTool(memoryManager)
		toolRegistry.Register(memoryTool)
		logger.Info("Memory tool registered")
	}

	// 初始化消息总线（先创建，以便 cron 和 agent 可以使用）
	fmt.Println("Creating message bus...")
	msgBus := bus.NewMessageBus(100, 100)
	fmt.Println("Message bus created")

	// 初始化 Cron 调度器（先创建，不带 handler，以便可以注册工具）
	var cronScheduler *cron.CronScheduler
	if cfg.Tools.Cron.Enabled {
		fmt.Println("Creating cron scheduler (for tool registration)...")
		cronStorePath := cfg.ResolvePath("cron/jobs.json")
		// 先创建不带 handler 的调度器
		cronScheduler = cron.NewCronScheduler(cronStorePath, nil)
		// 创建适配器和 CronTool，然后注册到工具注册表
		adapter := &cronSchedulerAdapter{scheduler: cronScheduler}
		cronTool := tools.NewCronTool(adapter)
		toolRegistry.Register(cronTool)
		logger.Info("Cron tool registered")
	}

	// 初始化 Agent（带增强功能）
	var maxTokens int = 4096 // 默认值
	if defaultProviderCfg, ok := cfg.LLM.Providers[cfg.LLM.DefaultProvider]; ok {
		if defaultProviderCfg.MaxTokens > 0 {
			maxTokens = defaultProviderCfg.MaxTokens
		}
	}
	tokenManager := agent.NewTokenManager(maxTokens * 10) // 10x 预算
	traceManager := agent.NewTraceManager(true) // 启用追踪

	// 创建并组合 Hook
	fmt.Println("Setting up agent hooks...")
	hook := agent.NewCompositeHook(
		agent.NewTraceHook(traceManager),
		agent.NewProgressHook(msgBus),
	)
	fmt.Println("Agent hooks configured")

	fmt.Println("Creating agent...")
	agentOpts := []agent.AgentLoopOption{
		agent.WithTokenManagerForLoop(tokenManager),
		agent.WithTraceManagerForLoop(traceManager),
		agent.WithMessageBusForLoop(msgBus),
		agent.WithHookForLoop(hook),
	}

	if memoryManager != nil {
		agentOpts = append(agentOpts, agent.WithMemoryManagerForLoop(memoryManager))
		logger.Info("Memory manager integrated into agent")
	}

	aiAgent := agent.NewAgentLoop(
		cfg,
		providerMgr,
		sessionMgr,
		toolRegistry,
		agentOpts...,
	)
	fmt.Println("Agent created")

	// 启动 Agent 的总线监听
	_ = aiAgent.Start()
	fmt.Println("Agent started with message bus and hooks")

	// 完成 Cron 调度器设置（设置 handler 并启动）
	if cfg.Tools.Cron.Enabled && cronScheduler != nil {
		fmt.Println("Finalizing cron scheduler setup...")
		cronIntegration := cron.NewCronIntegration(aiAgent, sessionMgr, msgBus)

		// 如果有记忆管理器，添加到集成中并注册系统任务
		if memoryManager != nil {
			cronIntegration.WithMemoryManager(memoryManager)

			// 注册 Dream 处理任务
			if cfg.Memory.DreamInterval != "" {
				fmt.Printf("Registering dream job with interval: %s\n", cfg.Memory.DreamInterval)
				dreamJob := cron.CronJob{
					ID:               "dream-process",
					Name:             "dream-process",
					Enabled:          true,
					Schedule: cron.CronSchedule{
						Kind: cron.ScheduleKindCron,
						Expr: cfg.Memory.DreamInterval,
					},
					Payload: cron.CronPayload{
						Kind:    cron.PayloadKindSystemEvent,
						Message: "dream-process",
					},
					State: cron.CronJobState{},
				}
				cronScheduler.RegisterSystemJob(dreamJob)
				logger.Info("Dream system job registered")
			}

			// 注册 AutoCompact 任务（每小时一次）
			if cfg.Memory.SessionTTLMinutes > 0 {
				compactJob := cron.CronJob{
					ID:               "autocompact-process",
					Name:             "autocompact-process",
					Enabled:          true,
					Schedule: cron.CronSchedule{
						Kind: cron.ScheduleKindCron,
						Expr: "0 * * * *",
					},
					Payload: cron.CronPayload{
						Kind:    cron.PayloadKindSystemEvent,
						Message: "autocompact-process",
					},
					State: cron.CronJobState{},
				}
				cronScheduler.RegisterSystemJob(compactJob)
				logger.Info("AutoCompact system job registered")
			}
		}

		// 为调度器设置 handler
		cronScheduler.SetHandler(cronIntegration.JobHandler)

		// 启动调度器
		if err := cronScheduler.Start(); err != nil {
			logger.Warn("Failed to start cron scheduler", logger.ErrorField(err))
		} else {
			fmt.Println("✅ Cron scheduler started")
		}
		logger.Info("Cron scheduler ready")
	}

	// 检查 LLM 配置
	hasAPIKey := false
	for name, providerCfg := range cfg.LLM.Providers {
		if providerCfg.APIKey != "" {
			fmt.Printf("✅ %s provider configured\n", name)
			hasAPIKey = true
		}
	}
	if !hasAPIKey {
		fmt.Println("")
		fmt.Println("⚠️ ⚠️ ⚠️ WARNING: NO LLM API KEYS CONFIGURED! ⚠️ ⚠️ ⚠️")
		fmt.Println("Please edit neoray.toml and add your API key")
		fmt.Println("")
	} else {
		fmt.Println("")
	}

	fmt.Println("Creating channel manager...")
	// 初始化频道管理器
	channelMgr := channel.NewManager(cfg, aiAgent, sessionMgr, msgBus)

	// 注册飞书频道
	fmt.Printf("Feishu config:\n")
	fmt.Printf("  Enabled: %v\n", cfg.Channels.Feishu.Enabled)
	fmt.Printf("  AppID: %s\n", cfg.Channels.Feishu.AppID)
	fmt.Printf("  AppSecret present: %v\n", cfg.Channels.Feishu.AppSecret != "")

	feishuConfig := &channel.FeishuConfig{
		AppID:            cfg.Channels.Feishu.AppID,
		AppSecret:        cfg.Channels.Feishu.AppSecret,
		Enabled:          cfg.Channels.Feishu.Enabled,
		VerificationToken: cfg.Channels.Feishu.VerificationToken,
		EncryptKey:       cfg.Channels.Feishu.EncryptKey,
		Domain:           cfg.Channels.Feishu.Domain,
		GroupPolicy:      cfg.Channels.Feishu.GroupPolicy,
		ReplyToMessage:   cfg.Channels.Feishu.ReplyToMessage,
		TopicIsolation:   cfg.Channels.Feishu.TopicIsolation,
		ReactEmoji:       cfg.Channels.Feishu.ReactEmoji,
		DoneEmoji:        cfg.Channels.Feishu.DoneEmoji,
		ToolHintPrefix:   cfg.Channels.Feishu.ToolHintPrefix,
		Streaming:        cfg.Channels.Feishu.Streaming,
	}
	channelMgr.RegisterChannel(channel.NewFeishuChannel(feishuConfig, cfg, aiAgent, sessionMgr))

	fmt.Println("Creating API server...")
	// 初始化 API 服务器
	apiServer := api.NewServer(cfg, aiAgent, sessionMgr, channelMgr, msgBus)
	fmt.Println("API server created")

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Starting message bus...")
	// 启动消息总线
	if err := msgBus.Start(); err != nil {
		logger.Error("Failed to start message bus", logger.ErrorField(err))
	}
	fmt.Println("Message bus started")

	fmt.Println("Starting API server...")
	// 启动 API 服务器
	if err := apiServer.Start(); err != nil {
		logger.Error("Failed to start API server", logger.ErrorField(err))
	} else {
		logger.Info("API server started",
			logger.String("host", cfg.Server.Host),
			logger.Int("port", cfg.Server.Port),
		)
		fmt.Printf("✅ Server running on http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
		fmt.Println("   Press Ctrl+C to stop")
	}

	fmt.Println("Starting channels...")
	// 启动频道
	if err := channelMgr.StartAll(); err != nil {
		logger.Error("Failed to start channels", logger.ErrorField(err))
	}
	fmt.Println("Channels started")

	fmt.Printf("useTUI flag: %v\n", *useTUI)

	if !*useTUI {
		// 仅运行服务器模式（默认）
		fmt.Println("Running in server-only mode (default)")
		fmt.Println("Use --tui flag to enable TUI mode")
		logger.Info("Running in server-only mode")
		<-sigChan
	} else {
		// 启动 TUI 模式
		fmt.Println("Starting TUI mode...")
		logger.Info("Running in TUI mode")

		// TUI 在主线程运行
		app := tui.NewTUI(aiAgent, sessionMgr)
		fmt.Println("TUI app created, calling Run()...")
		if err := app.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI Error: %v\n", err)
		}
		fmt.Println("TUI exited")
	}

	// 优雅关闭
	logger.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 停止 Cron 调度器
	if cronScheduler != nil {
		cronScheduler.Stop()
		logger.Info("Cron scheduler stopped")
	}

	// 停止消息总线
	_ = msgBus.Stop()

	// 停止频道
	channelMgr.StopAll()

	// 停止 API 服务器
	if err := apiServer.Stop(shutdownCtx); err != nil {
		logger.Warn("Error stopping API server", logger.ErrorField(err))
	}

	logger.Info("Shutdown complete")
}

func initProviders(cfg *config.Config) *provider.ProviderManager {
	var defaultProvider provider.Provider

	providerMgr := provider.NewProviderManager(nil)
	factory := provider.NewDefaultProviderFactory(cfg)

	fmt.Printf("Number of providers in config: %d\n", len(cfg.LLM.Providers))

	// 遍历所有配置的提供商
	for name, providerConfig := range cfg.LLM.Providers {
		fmt.Printf("Found provider: %s\n", name)
		fmt.Printf("  APIKey present: %s\n", func() string {
			if providerConfig.APIKey != "" {
				return "YES (length: " + fmt.Sprintf("%d", len(providerConfig.APIKey)) + ")"
			}
			return "NO"
		}())
		fmt.Printf("  APIFormat: %s\n", providerConfig.APIFormat)
		fmt.Printf("  APIURL: %s\n", providerConfig.APIURL)
		fmt.Printf("  Model: %s\n", providerConfig.Model)

		if providerConfig.APIKey == "" {
			logger.Warn("Provider configured but no API key", logger.String("provider", name))
			continue
		}

		// 复制配置，避免循环变量重用问题
		cfgCopy := providerConfig

		// 通过工厂创建 provider（内部按 api_format 路由）
		p, err := factory.CreateProvider(name, &cfgCopy)
		if err != nil {
			logger.Warn("Failed to create provider",
				logger.String("name", name),
				logger.String("error", err.Error()),
			)
			continue
		}
		logger.Info("Provider registered", logger.String("name", name), logger.String("api_format", cfgCopy.APIFormat))

		providerMgr.RegisterProvider(name, p)

		// 如果是默认提供商或还没设置默认提供商，设置为默认
		if defaultProvider == nil || name == cfg.LLM.DefaultProvider {
			defaultProvider = p
		}
	}

	// 如果配置了 fallback 模型，用 FallbackProvider 包装默认提供商
	if defaultProvider != nil {
		fallbackConfigs := factory.BuildFallbackConfigs()
		if len(fallbackConfigs) > 0 {
			logger.Info("Wrapping default provider with fallback",
				logger.String("provider", defaultProvider.Name()),
				logger.Int("fallback_count", len(fallbackConfigs)),
			)
			defaultProvider = factory.CreateFallbackProvider(defaultProvider, fallbackConfigs)
		}
	}

	providerMgr.SetDefaultProvider(defaultProvider)

	if defaultProvider == nil {
		logger.Warn("No LLM provider configured")
	} else {
		logger.Info("Default provider", logger.String("name", defaultProvider.Name()))
	}

	return providerMgr
}
