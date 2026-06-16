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
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/tools"
	"neoray/internal/tui"
)

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
	// 注册新工具
	toolRegistry.Register(tools.NewFindFilesTool())
	logger.Info("FindFiles tool registered")
	toolRegistry.Register(tools.NewGrepTool())
	logger.Info("Grep tool registered")
	toolRegistry.Register(tools.NewApplyPatchTool())
	logger.Info("ApplyPatch tool registered")
	toolRegistry.Register(tools.NewWebSearchTool())
	logger.Info("WebSearch tool registered")
	toolRegistry.Register(tools.NewWebFetchTool())
	logger.Info("WebFetch tool registered")
	logger.Info("Tool registry initialized", logger.Int("tool_count", len(toolRegistry.List())))

	// 初始化会话存储和管理器（使用文件存储）
	sessionDir := cfg.ResolvePath("sessions")
	var sessionStore session.Store
	fileStore, err := session.NewFileStore(sessionDir)
	if err != nil {
		logger.Warn("Failed to create file store, falling back to memory store", logger.ErrorField(err))
		sessionStore = session.NewMemoryStore()
	} else {
		sessionStore = fileStore
	}
	fmt.Println("Creating session manager...")
	sessionMgr := session.NewManager(cfg, sessionStore)

	fmt.Println("Initializing providers...")
	// 初始化 LLM 提供商
	providerMgr := initProviders(cfg)

	// 初始化 Agent（带增强功能）
	var maxTokens int = 4096 // 默认值
	if defaultProvider, ok := cfg.LLM.Providers[cfg.LLM.DefaultProvider]; ok {
		if defaultProvider.MaxTokens > 0 {
			maxTokens = defaultProvider.MaxTokens
		}
	}
	tokenManager := agent.NewTokenManager(maxTokens * 10) // 10x 预算
	traceManager := agent.NewTraceManager(true) // 启用追踪

	fmt.Println("Creating agent...")
	aiAgent := agent.NewAgent(
		cfg,
		providerMgr,
		sessionMgr,
		toolRegistry,
		agent.WithTokenManager(tokenManager),
		agent.WithTraceManager(traceManager),
	)
	fmt.Println("Agent created")

	// 初始化消息总线
	fmt.Println("Creating message bus...")
	msgBus := bus.NewMessageBus(100, 100)
	fmt.Println("Message bus created")

	// 创建并组合 Hook
	fmt.Println("Setting up agent hooks...")
	hook := agent.NewCompositeHook(
		agent.NewTraceHook(traceManager),
		agent.NewProgressHook(msgBus),
	)
	fmt.Println("Agent hooks configured")

	// 用总线更新 Agent
	aiAgent = agent.NewAgent(
		cfg,
		providerMgr,
		sessionMgr,
		toolRegistry,
		agent.WithTokenManager(tokenManager),
		agent.WithTraceManager(traceManager),
		agent.WithMessageBus(msgBus),
		agent.WithHook(hook),
	)
	// 启动 Agent 的总线监听
	_ = aiAgent.Start()
	fmt.Println("Agent started with message bus and hooks")

	// 初始化 Cron 调度器
	var cronScheduler *cron.CronScheduler
	if cfg.Tools.Cron.Enabled {
		fmt.Println("Creating cron scheduler...")
		cronStorePath := cfg.ResolvePath("cron/jobs.json")
		cronIntegration := cron.NewCronIntegration(aiAgent, sessionMgr, msgBus)
		cronScheduler = cron.NewCronScheduler(cronStorePath, cronIntegration.JobHandler)
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
		fmt.Println("Please edit config.toml and add your API key")
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

		// 根据 api_format 选择提供商实现
		var p provider.Provider
		apiFormat := cfgCopy.APIFormat
		if apiFormat == "" {
			apiFormat = "openai" // 默认使用 OpenAI 格式
		}

		switch apiFormat {
		case "anthropic":
			p = provider.NewAnthropicProvider(name, &cfgCopy)
			logger.Info("Provider registered (Anthropic format)", logger.String("name", name))
		case "openai":
			fallthrough
		default:
			p = provider.NewGenericProvider(name, &cfgCopy)
			logger.Info("Provider registered (OpenAI format)", logger.String("name", name))
		}

		providerMgr.RegisterProvider(name, p)

		// 如果是默认提供商或还没设置默认提供商，设置为默认
		if defaultProvider == nil || name == cfg.LLM.DefaultProvider {
			defaultProvider = p
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
