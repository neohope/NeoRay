package provider

import (
	"fmt"
	"strings"

	"neoray/internal/config"
)

// ProviderFactoryFunc 从配置创建 provider，name 为配置中的 provider 名称
type ProviderFactoryFunc func(name string, cfg *config.ProviderConfig) (Provider, error)

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

// CreateProvider 创建 provider。匹配顺序：cfg.APIFormat 精确匹配 → model name 启发式 → 默认 openai_compat
func (f *DefaultProviderFactory) CreateProvider(providerName string, cfg *config.ProviderConfig) (Provider, error) {
	// 优先按 api_format 显式匹配
	apiFormat := cfg.APIFormat
	if apiFormat == "" {
		apiFormat = "openai" // 默认使用 OpenAI 格式
	}
	if factory, ok := f.factories[apiFormat]; ok {
		return factory(providerName, cfg)
	}

	// 尝试根据模型名称启发式匹配
	if cfg.Model != "" {
		if containsAny(cfg.Model, "claude") {
			if factory, ok := f.factories["anthropic"]; ok {
				return factory(providerName, cfg)
			}
		}
		if containsAny(cfg.Model, "gpt", "openai") {
			if factory, ok := f.factories["openai"]; ok {
				return factory(providerName, cfg)
			}
		}
	}

	// 默认使用 OpenAI 兼容
	if factory, ok := f.factories["openai_compat"]; ok {
		return factory(providerName, cfg)
	}

	return nil, fmt.Errorf("no provider factory found for %s (api_format=%s)", providerName, apiFormat)
}

// BuildFallbackConfigs 将配置中的 FallbackModelConfig 转换为 FallbackConfig 列表
func (f *DefaultProviderFactory) BuildFallbackConfigs() []FallbackConfig {
	llmCfg := f.config.LLM
	if len(llmCfg.FallbackModels) == 0 {
		return nil
	}
	configs := make([]FallbackConfig, 0, len(llmCfg.FallbackModels))
	for _, fb := range llmCfg.FallbackModels {
		configs = append(configs, FallbackConfig{
			Model:            fb.Model,
			Provider:         fb.Provider,
			MaxTokens:        fb.MaxTokens,
			Temperature:      fb.Temperature,
			ReasoningEffort: fb.ReasoningEffort,
		})
	}
	return configs
}

// CreateFallbackProvider 创建带 fallback 的 provider
func (f *DefaultProviderFactory) CreateFallbackProvider(
	primary Provider,
	fallbackConfigs []FallbackConfig,
) Provider {
	return NewFallbackProvider(
		primary.Name()+"_with_fallback",
		primary,
		fallbackConfigs,
		f.createFallbackProviderFromConfig,
	)
}

// createFallbackProviderFromConfig 从 FallbackConfig 创建 provider
func (f *DefaultProviderFactory) createFallbackProviderFromConfig(fbConfig FallbackConfig) (Provider, error) {
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
func NewAnthropicProviderFromConfig(name string, cfg *config.ProviderConfig) (Provider, error) {
	p := NewAnthropicProvider(name, cfg)
	return p, nil
}

// NewOpenAICompatProviderFromConfig 从配置创建 OpenAI 兼容 provider
func NewOpenAICompatProviderFromConfig(name string, cfg *config.ProviderConfig) (Provider, error) {
	p := NewGenericProvider(name, cfg)
	return p, nil
}

// containsAny checks if s contains any of the substrings (case-insensitive).
func containsAny(s string, substrs ...string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrs {
		if strings.Contains(s, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
