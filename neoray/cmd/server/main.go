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
	"neoray/internal/channel"
	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/provider"
	"neoray/internal/session"
	"neoray/internal/tools"
	"neoray/internal/tui"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "", "path to config file")
	noTUI := flag.Bool("no-tui", false, "disable TUI mode and run API server only")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := logger.Init(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting neoray",
		logger.String("name", cfg.App.Name),
		logger.String("version", cfg.App.Version),
		logger.String("env", cfg.App.Env),
		logger.String("home_dir", cfg.HomeDir),
	)

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
	sessionMgr := session.NewManager(cfg, sessionStore)

	// 初始化 LLM 提供商
	providerMgr := initProviders(cfg)

	// 初始化 Agent
	aiAgent := agent.NewAgent(cfg, providerMgr, sessionMgr, toolRegistry)

	// 初始化 API 服务器
	apiServer := api.NewServer(cfg, aiAgent, sessionMgr)

	// 初始化频道管理器
	channelMgr := channel.NewManager(cfg, aiAgent, sessionMgr)

	// 注册飞书频道
	feishuConfig := &channel.FeishuConfig{
		AppID:     cfg.Channels.Feishu.AppID,
		AppSecret: cfg.Channels.Feishu.AppSecret,
		Enabled:   cfg.Channels.Feishu.Enabled,
	}
	channelMgr.RegisterChannel(channel.NewFeishuChannel(feishuConfig, cfg))

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 API 服务器
	if err := apiServer.Start(); err != nil {
		logger.Error("Failed to start API server", logger.ErrorField(err))
	} else {
		logger.Info("API server started",
			logger.String("host", cfg.Server.Host),
			logger.Int("port", cfg.Server.Port),
		)
	}

	// 启动频道
	if err := channelMgr.StartAll(); err != nil {
		logger.Error("Failed to start channels", logger.ErrorField(err))
	}

	if *noTUI {
		// 仅运行服务器模式
		logger.Info("Running in server-only mode (TUI disabled)")
		<-sigChan
	} else {
		// 启动 TUI 的同时保持 API 服务器运行
		go func() {
			app := tui.NewTUI(aiAgent, sessionMgr)
			if err := app.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "TUI Error: %v\n", err)
			}
			// TUI 退出时发送信号
			sigChan <- syscall.SIGTERM
		}()

		<-sigChan
	}

	// 优雅关闭
	logger.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	if cfg.LLM.Anthropic.APIKey != "" {
		anthropicProvider := provider.NewAnthropicProvider(&cfg.LLM.Anthropic)
		if cfg.LLM.DefaultProvider == "anthropic" {
			defaultProvider = anthropicProvider
		}
		logger.Info("Anthropic provider registered")
	}

	if cfg.LLM.OpenAI.APIKey != "" {
		openaiProvider := provider.NewOpenAIProvider(&cfg.LLM.OpenAI)
		if cfg.LLM.DefaultProvider == "openai" || defaultProvider == nil {
			defaultProvider = openaiProvider
		}
		logger.Info("OpenAI provider registered")
	}

	if defaultProvider == nil && cfg.LLM.Anthropic.APIKey != "" {
		defaultProvider = provider.NewAnthropicProvider(&cfg.LLM.Anthropic)
	} else if defaultProvider == nil && cfg.LLM.OpenAI.APIKey != "" {
		defaultProvider = provider.NewOpenAIProvider(&cfg.LLM.OpenAI)
	}

	providerMgr := provider.NewProviderManager(defaultProvider)
	if cfg.LLM.Anthropic.APIKey != "" {
		providerMgr.RegisterProvider("anthropic", provider.NewAnthropicProvider(&cfg.LLM.Anthropic))
	}
	if cfg.LLM.OpenAI.APIKey != "" {
		providerMgr.RegisterProvider("openai", provider.NewOpenAIProvider(&cfg.LLM.OpenAI))
	}

	if defaultProvider == nil {
		logger.Warn("No LLM provider configured")
	}

	return providerMgr
}
