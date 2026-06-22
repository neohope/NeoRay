package provider

import (
	"context"
	"errors"
	"io"
)

// Message LLM 消息
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
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
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	Tools      []Tool    `json:"tools,omitempty"`
	MaxTokens  int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream     bool      `json:"stream,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
}

// StreamChatResponse 流式聊天响应
type StreamChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Error        error
}

// Provider LLM 提供商接口
type Provider interface {
	// Name 提供商名称
	Name() string
	// Chat 发送聊天请求
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// ChatStream 流式聊天
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error)
}

// StreamToolProvider 支持流式工具调用的提供商
type StreamToolProvider interface {
	Provider
	// ChatStreamWithTools 流式聊天（带工具调用支持）
	ChatStreamWithTools(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error)
}

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
