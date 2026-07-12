package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"neoray/internal/logger"
)

const (
	primaryFailureThreshold = 3
	primaryCooldownSeconds  = 60
)

// FallbackProviderFactory 从 FallbackConfig 创建 Provider 的函数类型
type FallbackProviderFactory func(cfg FallbackConfig) (Provider, error)

// FallbackProvider 支持 Fallback 的提供商包装器
type FallbackProvider struct {
	name            string
	primary         Provider
	fallbackConfigs []FallbackConfig
	providerFactory FallbackProviderFactory
	generation      GenerationSettings

	// Circuit breaker state
	mu               sync.RWMutex
	primaryFailures  int
	primaryTrippedAt time.Time
}

// NewFallbackProvider 创建 Fallback 提供商
func NewFallbackProvider(
	name string,
	primary Provider,
	fallbackConfigs []FallbackConfig,
	providerFactory FallbackProviderFactory,
) *FallbackProvider {
	return &FallbackProvider{
		name:            name,
		primary:         primary,
		fallbackConfigs: fallbackConfigs,
		providerFactory: providerFactory,
		generation:      primary.GetGenerationSettings(),
	}
}

// Name 提供商名称
func (p *FallbackProvider) Name() string {
	return p.name
}

// GetGenerationSettings 获取生成设置
func (p *FallbackProvider) GetGenerationSettings() GenerationSettings {
	return p.generation
}

// SetGenerationSettings 设置生成设置
func (p *FallbackProvider) SetGenerationSettings(settings GenerationSettings) {
	p.generation = settings
	p.primary.SetGenerationSettings(settings)
}

// GetDefaultModel 获取默认模型
func (p *FallbackProvider) GetDefaultModel() string {
	return p.primary.GetDefaultModel()
}

// primaryAvailable 检查主提供商是否可用（熔断器）
func (p *FallbackProvider) primaryAvailable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.primaryTrippedAt.IsZero() {
		return true
	}

	if time.Since(p.primaryTrippedAt) >= primaryCooldownSeconds*time.Second {
		return true
	}

	return false
}

// recordPrimarySuccess 记录主提供商成功
func (p *FallbackProvider) recordPrimarySuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.primaryFailures = 0
	p.primaryTrippedAt = time.Time{}
}

// recordPrimaryFailure 记录主提供商失败
func (p *FallbackProvider) recordPrimaryFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.primaryFailures++
	if p.primaryFailures >= primaryFailureThreshold {
		p.primaryTrippedAt = time.Now()
		logger.Warn("Primary provider circuit opened",
			logger.String("provider", p.primary.Name()),
			logger.Int("failures", p.primaryFailures),
		)
	}
}

// Chat 发送聊天请求
func (p *FallbackProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// 如果没有 fallback 配置，直接使用主提供商
	if len(p.fallbackConfigs) == 0 {
		return p.primary.Chat(ctx, req)
	}

	return p.tryChatWithFallback(ctx, req, nil)
}

// ChatStream 流式聊天
func (p *FallbackProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error) {
	// 如果没有 fallback 配置，直接使用主提供商
	if len(p.fallbackConfigs) == 0 {
		return p.primary.ChatStream(ctx, req)
	}

	resultChan := make(chan StreamChatResponse)
	hasStreamed := false

	go func() {
		defer close(resultChan)

		// 尝试主提供商
		if p.primaryAvailable() {
			streamChan, err := p.primary.ChatStream(ctx, req)
			if err == nil {
				// 流式接收响应
				for resp := range streamChan {
					if resp.Error != nil {
						// 如果已经输出了内容，就不 fallback
						if hasStreamed {
							resultChan <- resp
							return
						}
						// 记录失败并尝试 fallback
						p.recordPrimaryFailure()
						logger.Warn("Primary stream failed, trying fallback",
							logger.String("error", resp.Error.Error()),
						)
						break
					}

					if resp.Content != "" {
						hasStreamed = true
					}

					resultChan <- resp

					if resp.FinishReason != "" && resp.FinishReason != "error" {
						p.recordPrimarySuccess()
						return
					}
				}
				// 如果循环结束但没有成功返回，继续尝试 fallback
			} else {
				// 创建失败，直接尝试 fallback
				p.recordPrimaryFailure()
				logger.Warn("Primary provider failed, trying fallback",
					logger.String("error", err.Error()),
				)
			}
		} else {
			logger.Debug("Primary provider circuit is open, skipping to fallback")
		}

		// 尝试 fallback 提供商
		primaryModel := req.Model
		for i, fallbackConfig := range p.fallbackConfigs {
			select {
			case <-ctx.Done():
				resultChan <- StreamChatResponse{Error: ctx.Err()}
				return
			default:
			}

			logger.Info("Trying fallback provider",
				logger.Int("index", i),
				logger.String("model", fallbackConfig.Model),
				logger.String("provider", fallbackConfig.Provider),
			)

			// 创建 fallback 提供商
			fallbackProvider, err := p.providerFactory(fallbackConfig)
			if err != nil {
				logger.Warn("Failed to create fallback provider",
					logger.String("error", err.Error()),
				)
				continue
			}

			// 创建请求副本，避免修改调用方的原始请求
			fallbackReq := *req
			fallbackReq.Model = fallbackConfig.Model
			if fallbackConfig.MaxTokens > 0 {
				fallbackReq.MaxTokens = fallbackConfig.MaxTokens
			}
			if fallbackConfig.Temperature > 0 {
				fallbackReq.Temperature = fallbackConfig.Temperature
			}
			if fallbackConfig.ReasoningEffort != "" {
				fallbackReq.ReasoningEffort = fallbackConfig.ReasoningEffort
			}

			// 尝试 fallback 流式请求
			streamChan, err := fallbackProvider.ChatStream(ctx, &fallbackReq)

			if err != nil {
				logger.Warn("Fallback provider failed",
					logger.Int("index", i),
					logger.String("error", err.Error()),
				)
				continue
			}

			fallbackSuccess := false
			for resp := range streamChan {
				if resp.Error != nil {
					logger.Warn("Fallback stream error",
						logger.Int("index", i),
						logger.String("error", resp.Error.Error()),
					)
					break
				}

				if resp.Content != "" {
					hasStreamed = true
				}

				resultChan <- resp

				if resp.FinishReason != "" && resp.FinishReason != "error" {
					fallbackSuccess = true
					logger.Info("Fallback provider succeeded",
						logger.Int("index", i),
						logger.String("model", fallbackConfig.Model),
					)
					return
				}
			}

			if fallbackSuccess {
				return
			}
		}

		// 所有 fallback 都失败了
		if primaryModel == "" {
			primaryModel = p.primary.GetDefaultModel()
		}
		resultChan <- StreamChatResponse{
			Error: fmt.Errorf("all fallback providers failed after primary %s failed", primaryModel),
		}
	}()

	return resultChan, nil
}

// tryChatWithFallback 尝试聊天，失败时使用 fallback
func (p *FallbackProvider) tryChatWithFallback(
	ctx context.Context,
	req *ChatRequest,
	hasStreamed *bool,
) (*ChatResponse, error) {
	primaryModel := req.Model
	if primaryModel == "" {
		primaryModel = p.primary.GetDefaultModel()
	}

	// 尝试主提供商
	if p.primaryAvailable() {
		resp, err := p.primary.Chat(ctx, req)
		if err == nil && resp.FinishReason != "error" {
			p.recordPrimarySuccess()
			return resp, nil
		}

		// 检查是否应该 fallback
		if (hasStreamed != nil && *hasStreamed) || (resp != nil && !ShouldFallbackError(resp)) {
			if err != nil {
				return nil, err
			}
			return resp, nil
		}

		// 记录失败
		p.recordPrimaryFailure()
		logger.Warn("Primary provider failed, trying fallbacks",
			logger.String("provider", p.primary.Name()),
			logger.String("model", primaryModel),
		)
	} else {
		logger.Debug("Primary provider circuit open, trying fallbacks directly")
	}

	// 尝试 fallback 提供商
	var lastResp *ChatResponse
	for i, fallbackConfig := range p.fallbackConfigs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		logger.Info("Trying fallback provider",
			logger.Int("index", i),
			logger.String("model", fallbackConfig.Model),
			logger.String("provider", fallbackConfig.Provider),
		)

		// 创建 fallback 提供商
		fallbackProvider, err := p.providerFactory(fallbackConfig)
		if err != nil {
			logger.Warn("Failed to create fallback provider",
				logger.String("error", err.Error()),
			)
			continue
		}

		// 创建请求副本，避免修改调用方的原始请求
		fallbackReq := *req
		fallbackReq.Model = fallbackConfig.Model
		if fallbackConfig.MaxTokens > 0 {
			fallbackReq.MaxTokens = fallbackConfig.MaxTokens
		}
		if fallbackConfig.Temperature > 0 {
			fallbackReq.Temperature = fallbackConfig.Temperature
		}
		if fallbackConfig.ReasoningEffort != "" {
			fallbackReq.ReasoningEffort = fallbackConfig.ReasoningEffort
		}

		// 尝试 fallback 请求
		resp, err := fallbackProvider.Chat(ctx, &fallbackReq)

		if err != nil {
			logger.Warn("Fallback provider failed",
				logger.Int("index", i),
				logger.String("error", err.Error()),
			)
			continue
		}

		if resp.FinishReason != "error" {
			logger.Info("Fallback provider succeeded",
				logger.Int("index", i),
				logger.String("model", fallbackConfig.Model),
			)
			return resp, nil
		}

		lastResp = resp
		logger.Warn("Fallback provider returned error",
			logger.Int("index", i),
			logger.String("finish_reason", resp.FinishReason),
		)
	}

	// 所有 fallback 都失败
	logger.Warn("All fallback providers failed", logger.Int("count", len(p.fallbackConfigs)))

	if lastResp != nil {
		return lastResp, nil
	}

	// 返回错误
	return &ChatResponse{
		Content:      fmt.Sprintf("Primary model %s circuit open and no fallbacks available", primaryModel),
		FinishReason: "error",
		ErrorType:    "no_fallback_available",
	}, nil
}

// HasFallbacks 检查是否配置了 fallback
func (p *FallbackProvider) HasFallbacks() bool {
	return len(p.fallbackConfigs) > 0
}

// GetPrimary 获取主提供商
func (p *FallbackProvider) GetPrimary() Provider {
	return p.primary
}
