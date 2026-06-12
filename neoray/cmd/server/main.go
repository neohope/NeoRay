package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"neoray/internal/agent"
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
	noTUI := flag.Bool("no-tui", false, "disable TUI mode")
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

	if *noTUI {
		logger.Infof("Server listening on %s:%d", cfg.Server.Host, cfg.Server.Port)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		return
	}

	// 启动 TUI
	app := tui.NewTUI(aiAgent, sessionMgr)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
