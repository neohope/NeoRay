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

	// 初始化会话存储和管理器
	sessionStore := session.NewMemoryStore()
	sessionMgr := session.NewManager(cfg, sessionStore)

	// 初始化 LLM 提供商
	providerMgr := initProviders(cfg)

	// 初始化 Agent
	aiAgent := agent.NewAgent(cfg, providerMgr, sessionMgr)

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
