package provider

import (
	"fmt"

	"neoray/internal/config"
	"neoray/internal/logger"
)

// ProviderFactoryFunc 从配置创建 provider
type ProviderFactoryFunc func(cfg *config.ProviderConfig) (Provider, error)

// DefaultProviderFactory 默认的 provider 工厂
type DefaultProviderFactory struct {
	factories map[string]ProviderFactoryFunc
	config    *config.Config
}

// NewDefaultProviderFactory 创建默认的 provider 工厂
func NewDefaultProviderFactory(cfg *config.Config) *DefaultProviderFactory {
	f := &DefaultProviderFactory{
		factories: make(map[string]ProviderFactoryFunc),
		config:    cfg,
	}

	// 注册内置的 provider 工厂
	f.RegisterFactory("anthropic", NewAnthropicProviderFromConfig)
	f.RegisterFactory("openai", NewOpenAICompatProviderFromConfig)
	f.RegisterFactory("openai_compat", NewOpenAICompatProviderFromConfig)

	return f
}

// RegisterFactory 注册 provider 工厂
func (f *DefaultProviderFactory) RegisterFactory(name string, factory ProviderFactoryFunc) {
	f.factories[name] = factory
}

// CreateProvider 创建 provider
func (f *DefaultProviderFactory) CreateProvider(name string, cfg *config.ProviderConfig) (Provider, error) {
	// 尝试找到合适的工厂
	var factory ProviderFactoryFunc
	var ok bool

	// 首先尝试精确匹配
	if factory, ok = f.factories[name]; ok {
		return factory(cfg)
	}

	// 尝试根据模型名称猜测
	if cfg.Model != "" {
		if containsAny(cfg.Model, "claude") {
			if factory, ok = f.factories["anthropic"]; ok {
				return factory(cfg)
			}
		}
		if containsAny(cfg.Model, "gpt", "openai") {
			if factory, ok = f.factories["openai"]; ok {
				return factory(cfg)
			}
		}
	}

	// 默认使用 OpenAI 兼容
	if factory, ok = f.factories["openai_compat"]; ok {
		return factory(cfg)
	}

	return nil, fmt.Errorf("no provider factory found for %s", name)
}

// CreateProviderFromConfig 从全局配置创建 provider
func (f *DefaultProviderFactory) CreateProviderFromConfig() (Provider, error) {
	llmCfg := f.config.LLM

	// 获取默认 provider 名称
	defaultProviderName := llmCfg.DefaultProvider
	if defaultProviderName == "" {
		defaultProviderName = "anthropic" // 默认
	}

	// 查找默认 provider 配置
	var primaryProvider Provider
	var primaryProviderName string

	// 先尝试找 anthropic 配置，否则找第一个可用的
	for name, cfg := range llmCfg.Providers {
		if cfg.APIKey != "" || cfg.APIURL != "" {
			if name == defaultProviderName || (defaultProviderName == "" && name == "anthropic") {
				provider, err := f.CreateProvider(name, &cfg)
				if err != nil {
					logger.Warn("Failed to create provider", logger.String("name", name), logger.String("error", err.Error()))
					continue
				}
				primaryProvider = provider
				primaryProviderName = name
				logger.Info("Created primary provider", logger.String("name", name), logger.String("model", cfg.Model))
				break
			}
		}
	}

	// 如果还没有找到，尝试找任何一个有 API Key 的
	if primaryProvider == nil {
		for name, cfg := range llmCfg.Providers {
			if cfg.APIKey != "" {
				provider, err := f.CreateProvider(name, &cfg)
				if err == nil {
					primaryProvider = provider
					primaryProviderName = name
					logger.Info("Created primary provider (fallback)", logger.String("name", name), logger.String("model", cfg.Model))
					break
				}
			}
		}
	}

	if primaryProvider == nil {
		return nil, fmt.Errorf("no valid provider configuration found")
	}

	// 处理 fallback 配置
	if len(llmCfg.FallbackModels) > 0 {
		fallbackConfigs := make([]FallbackConfig, 0, len(llmCfg.FallbackModels))
		for _, fb := range llmCfg.FallbackModels {
			fallbackConfigs = append(fallbackConfigs, FallbackConfig{
				Model:            fb.Model,
				Provider:         fb.Provider,
				MaxTokens:        fb.MaxTokens,
				Temperature:      fb.Temperature,
				ReasoningEffort: fb.ReasoningEffort,
			})
		}

		if len(fallbackConfigs) > 0 {
			logger.Info("Creating fallback provider", logger.Int("fallback_count", len(fallbackConfigs)))
			return NewFallbackProvider(
				primaryProviderName+"_with_fallback",
				primaryProvider,
				fallbackConfigs,
				f.CreateFallbackProviderFromConfig,
			), nil
		}
	}

	return primaryProvider, nil
}

// CreateFallbackProvider 创建带 fallback 的 provider
func (f *DefaultProviderFactory) CreateFallbackProvider(
	primary Provider,
	fallbackConfigs []FallbackConfig,
) Provider {
	return NewFallbackProvider(
		"fallback_wrapper",
		primary,
		fallbackConfigs,
		f.CreateFallbackProviderFromConfig,
	)
}

// ProviderFactoryFunc 实现 - 从 FallbackConfig 创建 provider
func (f *DefaultProviderFactory) CreateFallbackProviderFromConfig(fbConfig FallbackConfig) (Provider, error) {
	// 查找或创建 provider 配置
	var cfg *config.ProviderConfig

	// 先从全局配置中查找
	if providerCfg, ok := f.config.LLM.Providers[fbConfig.Provider]; ok {
		cfg = &providerCfg
		// 覆盖模型
		if fbConfig.Model != "" {
			cfg.Model = fbConfig.Model
		}
		// 覆盖其他参数
		if fbConfig.MaxTokens > 0 {
			cfg.MaxTokens = fbConfig.MaxTokens
		}
		if fbConfig.Temperature > 0 {
			cfg.Temperature = fbConfig.Temperature
		}
		cfg.ReasoningEffort = fbConfig.ReasoningEffort
	} else {
		// 创建一个基本配置
		cfg = &config.ProviderConfig{
			Model:            fbConfig.Model,
			MaxTokens:        fbConfig.MaxTokens,
			Temperature:      fbConfig.Temperature,
			ReasoningEffort: fbConfig.ReasoningEffort,
		}
	}

	return f.CreateProvider(fbConfig.Provider, cfg)
}

// NewAnthropicProviderFromConfig 从配置创建 Anthropic provider
func NewAnthropicProviderFromConfig(cfg *config.ProviderConfig) (Provider, error) {
	provider := NewAnthropicProvider("anthropic", cfg)
	return provider, nil
}

// NewOpenAICompatProviderFromConfig 从配置创建 OpenAI 兼容 provider
func NewOpenAICompatProviderFromConfig(cfg *config.ProviderConfig) (Provider, error) {
	provider := NewGenericProvider("openai_compat", cfg)
	return provider, nil
}

// 辅助函数
func containsAny(s string, substrs ...string) bool {
	s = toLower(s)
	for _, substr := range substrs {
		if contains(s, toLower(substr)) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	// 简单的小写转换
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
