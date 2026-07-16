package provider

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

// Message LLM 消息
type Message struct {
	Role          string                 `json:"role"`
	Content       string                 `json:"content"`
	ToolCalls     []ToolCall             `json:"tool_calls,omitempty"`
	ThinkingBlocks []ThinkingBlock       `json:"thinking_blocks,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ThinkingBlock 思考块
type ThinkingBlock struct {
	Type      string `json:"type"`       // "thinking"
	Thinking  string `json:"thinking"`   // 思考内容
	Signature string `json:"signature"`  // 签名（可选）
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool 工具定义
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl 缓存控制
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model            string          `json:"model"`
	Messages         []Message       `json:"messages"`
	Tools            []Tool          `json:"tools,omitempty"`
	MaxTokens        int             `json:"max_tokens,omitempty"`
	Temperature      float64         `json:"temperature,omitempty"`
	Stream           bool            `json:"stream,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"` // "low", "medium", "high", "adaptive", "none"
	ToolChoice       string          `json:"tool_choice,omitempty"`
	CacheEnabled     bool            `json:"cache_enabled,omitempty"`
}

// Usage 使用量
type Usage struct {
	InputTokens            int `json:"input_tokens"`
	OutputTokens           int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CachedTokens           int `json:"cached_tokens,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content         string
	ToolCalls       []ToolCall
	ThinkingBlocks []ThinkingBlock
	FinishReason    string
	Usage          *Usage
	RetryAfter     time.Duration
	ErrorStatusCode int
	ErrorType      string
	ErrorCode      string
	ErrorShouldRetry bool
}

// StreamChatResponse 流式聊天响应
type StreamChatResponse struct {
	Content         string
	ThinkingDelta   string
	ToolCalls       []ToolCall
	FinishReason    string
	Error          error
}

// ProviderError 带重试信息的提供商错误
type ProviderError struct {
	Err             error
	RetryAfter      time.Duration
	ErrorShouldRetry bool
	ErrorType       string
	StatusCode      int
}

func (e *ProviderError) Error() string { return e.Err.Error() }
func (e *ProviderError) Unwrap() error { return e.Err }

// GenerationSettings 生成设置
type GenerationSettings struct {
	Temperature      float64
	MaxTokens        int
	ReasoningEffort string
}

// Provider LLM 提供商接口
type Provider interface {
	// Name 提供商名称
	Name() string
	// Chat 发送聊天请求
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// ChatStream 流式聊天
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error)
	// GetGenerationSettings 获取生成设置
	GetGenerationSettings() GenerationSettings
	// SetGenerationSettings 设置生成设置
	SetGenerationSettings(settings GenerationSettings)
	// GetDefaultModel 获取默认模型
	GetDefaultModel() string
}

// StreamToolProvider 支持流式工具调用的提供商
type StreamToolProvider interface {
	Provider
	// ChatStreamWithTools 流式聊天（带工具调用支持）
	ChatStreamWithTools(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error)
}

// RetryProvider 支持重试的提供商
type RetryProvider interface {
	Provider
	// ChatWithRetry 带重试的聊天
	ChatWithRetry(ctx context.Context, req *ChatRequest, retryMode string, onRetryWait func(string)) (*ChatResponse, error)
	// ChatStreamWithRetry 带重试的流式聊天
	ChatStreamWithRetry(ctx context.Context, req *ChatRequest, retryMode string, onRetryWait func(string)) (<-chan StreamChatResponse, error)
}

// FallbackConfig Fallback 配置
type FallbackConfig struct {
	Model            string
	Provider         string
	MaxTokens        int
	Temperature      float64
	ReasoningEffort string
}

// ProviderFactory 提供商工厂函数类型
type ProviderFactory func(config FallbackConfig) (Provider, error)

// FactoryProvider 提供商工厂
type FactoryProvider interface {
	CreateAnthropic() Provider
	CreateOpenAI() Provider
}

// ProviderManager 提供商管理器
type ProviderManager struct {
	defaultProvider Provider
	providers      map[string]Provider
}

// NewProviderManager 创建提供商管理器
func NewProviderManager(defaultProvider Provider) *ProviderManager {
	return &ProviderManager{
		defaultProvider: defaultProvider,
		providers:      make(map[string]Provider),
	}
}

// RegisterProvider 注册提供商
func (m *ProviderManager) RegisterProvider(name string, p Provider) {
	m.providers[name] = p
}

// GetProvider 获取提供商
func (m *ProviderManager) GetProvider(name string) (Provider, error) {
	if name == "" {
		return m.defaultProvider, nil
	}
	p, ok := m.providers[name]
	if !ok {
		return nil, errors.New("provider not found")
	}
	return p, nil
}

// DefaultProvider 获取默认提供商
func (m *ProviderManager) DefaultProvider() Provider {
	return m.defaultProvider
}

// SetDefaultProvider 设置默认提供商
func (m *ProviderManager) SetDefaultProvider(p Provider) {
	m.defaultProvider = p
}

// SetDefaultProviderByName 按名称设置默认提供商
func (m *ProviderManager) SetDefaultProviderByName(name string) error {
	if name == "" {
		return nil
	}
	p, ok := m.providers[name]
	if !ok {
		return errors.New("provider not found")
	}
	m.defaultProvider = p
	return nil
}

// ListProviders 列出所有已注册的提供商
func (m *ProviderManager) ListProviders() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// StreamReader 流读取器
type StreamReader interface {
	io.ReadCloser
	ReadResponse() (*StreamChatResponse, error)
}

// Error helper functions
func IsTransientError(resp *ChatResponse) bool {
	if resp == nil {
		return false
	}
	if resp.ErrorShouldRetry {
		return true
	}
	if resp.ErrorStatusCode >= 500 && resp.ErrorStatusCode < 600 {
		return true
	}
	if resp.ErrorStatusCode == 408 || resp.ErrorStatusCode == 409 || resp.ErrorStatusCode == 429 {
		if !IsNonRetryable429(resp) {
			return true
		}
	}
	transientTypes := map[string]bool{
		"timeout": true,
		"connection": true,
		"server_error": true,
		"rate_limit": true,
		"overloaded": true,
	}
	if transientTypes[resp.ErrorType] {
		return true
	}
	return false
}

func IsNonRetryable429(resp *ChatResponse) bool {
	if resp == nil {
		return false
	}
	nonRetryableTokens := []string{
		"insufficient_quota",
		"quota_exceeded",
		"quota_exhausted",
		"billing_hard_limit_reached",
		"insufficient_balance",
		"credit_balance_too_low",
		"billing_not_active",
		"payment_required",
	}
	for _, token := range nonRetryableTokens {
		if resp.ErrorType == token || resp.ErrorCode == token {
			return true
		}
	}
	return false
}

func IsArrearageError(resp *ChatResponse) bool {
	if resp == nil {
		return false
	}
	if resp.ErrorStatusCode == 402 {
		return true
	}
	return IsNonRetryable429(resp)
}

func ShouldFallbackError(resp *ChatResponse) bool {
	if resp == nil {
		return false
	}
	if resp.ErrorShouldRetry {
		return true
	}
	if resp.ErrorStatusCode >= 500 && resp.ErrorStatusCode < 600 {
		return true
	}
	if resp.ErrorStatusCode == 408 || resp.ErrorStatusCode == 409 || resp.ErrorStatusCode == 429 {
		return true
	}
	if resp.ErrorStatusCode == 400 || resp.ErrorStatusCode == 401 || resp.ErrorStatusCode == 403 || resp.ErrorStatusCode == 404 || resp.ErrorStatusCode == 422 {
		return false
	}
	nonFallbackTypes := map[string]bool{
		"authentication": true,
		"auth": true,
		"permission": true,
		"content_filter": true,
		"refusal": true,
		"context_length": true,
		"invalid_request": true,
	}
	if nonFallbackTypes[resp.ErrorType] {
		return false
	}
	fallbackTypes := map[string]bool{
		"timeout": true,
		"connection": true,
		"server_error": true,
		"rate_limit": true,
		"overloaded": true,
	}
	if fallbackTypes[resp.ErrorType] {
		return true
	}
	fallbackTokens := []string{
		"rate_limit", "too_many_requests", "overloaded",
		"server_error", "temporarily unavailable", "timeout",
		"insufficient_quota", "quota_exceeded", "quota_exhausted",
		"billing_hard_limit", "insufficient_balance", "balance", "out of credits",
	}
	for _, token := range fallbackTokens {
		if containsToken(resp.ErrorType, token) || containsToken(resp.ErrorCode, token) {
			return true
		}
	}
	return false
}

func containsToken(s, token string) bool {
	return len(s) >= len(token) && strings.Contains(s, token)
}
