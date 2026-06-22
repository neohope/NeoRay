# Provider 高级功能实现总结

## 概述

本文档总结了从 neonanobot 项目参考并在 neoray 项目中实现的 Provider 高级功能：

1. **Fallback Provider** - 主提供商失败时自动切换到备用提供商
2. **Prompt Cache** - Anthropic API 的提示缓存支持，降低成本和延迟
3. **Thinking Mode** - Claude 模型的深度思考模式支持

---

## 1. Fallback Provider (备用提供商)

### 功能特性

- **自动故障转移**：当主提供商返回错误时自动尝试备用提供商
- **熔断器模式**：主提供商连续失败后暂时禁用，避免资源浪费
- **配置灵活**：支持配置多个备用提供商，每个可以有不同的模型和参数
- **流式和非流式**：完全支持两种请求模式的 fallback

### 核心组件

```go
// FallbackProvider - 主包装器
type FallbackProvider struct {
    name            string
    primary         Provider
    fallbackConfigs []FallbackConfig
    providerFactory FallbackProviderFactory
    // ...
}

// FallbackConfig - 单个备用配置
type FallbackConfig struct {
    Model            string
    Provider         string
    MaxTokens        int
    Temperature      float64
    ReasoningEffort string
}
```

### 配置示例

```toml
[llm]
default_provider = "anthropic"

[[llm.fallback_models]]
model = "gpt-4"
provider = "openai"
max_tokens = 4096
temperature = 0.7

[[llm.fallback_models]]
model = "deepseek-chat"
provider = "deepseek"
```

### 使用方式

```go
factory := NewDefaultProviderFactory(config)
provider, err := factory.CreateProviderFromConfig()
// provider 已经包含了完整的 fallback 链

resp, err := provider.Chat(ctx, req)
// 会自动处理故障转移
```

---

## 2. Prompt Cache (提示缓存)

### 功能特性

- **降低成本**：缓存的 prompt token 价格更低
- **减少延迟**：缓存命中时响应更快
- **智能标记**：自动在合适的位置添加缓存标记
- **透明集成**：无需修改应用代码即可使用

### 核心实现

在 `AnthropicProvider` 中：

```go
// 请求头
httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

// 缓存标记
type anthropicCacheControl struct {
    Type string `json:"type"` // "ephemeral"
}

// 在系统消息和工具定义中应用缓存标记
```

### 使用量统计

```go
type Usage struct {
    InputTokens            int
    OutputTokens           int
    CacheCreationInputTokens int  // 创建缓存的 token
    CacheReadInputTokens     int  // 读取缓存的 token
    CachedTokens           int  // 归一化的缓存 token
}
```

### 配置

```toml
[llm.providers.anthropic]
prompt_cache_enabled = true
```

---

## 3. Thinking Mode (深度思考模式)

### 功能特性

- **多级思考**：支持 low/medium/high/adaptive 等级别
- **自适应模式**：Claude 自动决定是否需要深度思考
- **思考块支持**：完整的思考内容返回和流式传递
- **温度自动调整**：思考模式下自动设置 temperature=1

### 核心实现

```go
// 请求参数
type anthropicThinking struct {
    Type         string `json:"type"` // "enabled" 或 "adaptive"
    BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// 思考块
type ThinkingBlock struct {
    Type      string `json:"type"`
    Thinking  string `json:"thinking"`
    Signature string `json:"signature"`
}
```

### 配置选项

```toml
[llm.providers.anthropic]
reasoning_effort = "medium"  # low, medium, high, adaptive, none
```

### 请求使用

```go
req := &ChatRequest{
    Model:           "claude-3-opus-20240229",
    Messages:        messages,
    ReasoningEffort: "adaptive",  // 可覆盖默认配置
}
```

---

## 4. 增强的 Provider 接口

### 完整接口

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChatResponse, error)
    GetGenerationSettings() GenerationSettings
    SetGenerationSettings(settings GenerationSettings)
    GetDefaultModel() string
}
```

### 增强的请求和响应

```go
type ChatRequest struct {
    Model            string
    Messages         []Message
    Tools            []Tool
    MaxTokens        int
    Temperature      float64
    Stream           bool
    ReasoningEffort string   // 新增
    ToolChoice       string
    CacheEnabled     bool     // 新增
}

type ChatResponse struct {
    Content         string
    ToolCalls       []ToolCall
    ThinkingBlocks []ThinkingBlock  // 新增
    FinishReason    string
    Usage          *Usage            // 新增
    RetryAfter     time.Duration
    ErrorStatusCode int
    ErrorType      string
    ErrorCode      string
    ErrorShouldRetry bool
}
```

---

## 5. 配置结构更新

### 后端配置 (Go)

```go
type LLMConfig struct {
    DefaultProvider string
    Providers       map[string]ProviderConfig
    FallbackModels  []FallbackModelConfig  // 新增
}

type ProviderConfig struct {
    APIKey            string
    APIURL            string
    Model             string
    MaxTokens         int
    Temperature       float64
    Timeout           time.Duration
    APIFormat         string
    ReasoningEffort  string        // 新增
    PromptCacheEnabled bool        // 新增
}

type FallbackModelConfig struct {
    Model            string
    Provider         string
    MaxTokens        int
    Temperature      float64
    ReasoningEffort string
}
```

### 前端配置 (Dart)

```dart
@freezed
class LLMConfig with _$LLMConfig {
  const factory LLMConfig({
    // ... 现有字段
    @Default('none') String reasoningEffort,
    @Default(false) bool promptCacheEnabled,
    @Default([]) List<FallbackModelConfig> fallbackModels,
  }) = _LLMConfig;
}
```

---

## 6. 文件结构

### 新增和修改的文件

```
neoray/internal/provider/
├── provider.go          # 增强的接口和数据结构
├── anthropic.go         # 完整重写，支持缓存和思考模式
├── openai.go            # 更新，实现完整接口
├── fallback.go          # 新增，Fallback Provider
└── factory.go           # 新增，Provider 工厂

neoray/internal/config/
└── config.go            # 更新配置结构

neoray_app/lib/models/
└── app_config.dart      # 更新前端配置

config/
└── config_example.toml  # 新增，完整配置示例

docs/
└── PROVIDER_FEATURES_SUMMARY.md  # 本文档
```

---

## 7. 使用示例

### 完整的 Provider 链创建

```go
// 1. 加载配置
config := loadConfig()

// 2. 创建工厂
factory := NewDefaultProviderFactory(config)

// 3. 创建完整的 provider 链（包含 fallback）
provider, err := factory.CreateProviderFromConfig()
if err != nil {
    log.Fatal(err)
}

// 4. 使用（透明支持所有功能）
resp, err := provider.Chat(ctx, &ChatRequest{
    Model:            "claude-3-opus-20240229",
    Messages:         messages,
    ReasoningEffort: "adaptive",
    CacheEnabled:     true,
})
```

### 单独使用 Fallback Provider

```go
// 创建主提供商
primary := NewAnthropicProvider("anthropic", anthropicConfig)

// 配置 fallback
fallbacks := []FallbackConfig{
    {Model: "gpt-4", Provider: "openai", MaxTokens: 4096},
    {Model: "deepseek-chat", Provider: "deepseek", MaxTokens: 4096},
}

// 创建包装器
fallbackProvider := NewFallbackProvider(
    "anthropic_with_fallback",
    primary,
    fallbacks,
    myFactoryFunc,
)
```

---

## 8. 错误处理

### 可重试错误

- 网络超时
- 5xx 服务器错误
- 429 速率限制（非配额耗尽）

### 不可重试错误

- 400 无效请求
- 401/403 认证错误
- 404 未找到
- 配额耗尽/欠费

### Fallback 判断

```go
func ShouldFallbackError(resp *ChatResponse) bool {
    // 基于错误类型和状态码判断是否应该尝试 fallback
}
```

---

## 9. 后续优化建议

1. **指标和监控**：添加 fallback 触发次数、缓存命中率等指标
2. **动态调整**：根据运行时性能动态调整 fallback 顺序
3. **更多 Provider**：扩展 thinking mode 支持其他模型（DeepSeek 等）
4. **前端 UI**：添加配置页面，让用户可以方便地调整这些高级选项
5. **单元测试**：完善测试覆盖

---

## 总结

本次实现完成了 Provider 高级功能的完整移植和适配：

✅ **Fallback Provider** - 高可用性保障
✅ **Prompt Cache** - 成本和性能优化
✅ **Thinking Mode** - 复杂推理能力
✅ **配置集成** - 完整的配置支持
✅ **前端更新** - 配置模型扩展
✅ **示例配置** - 完整的配置文档

所有功能都设计为可配置和向后兼容，用户可以根据需要选择启用哪些功能。
