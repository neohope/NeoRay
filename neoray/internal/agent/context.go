package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"neoray/internal/config"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// ContextStrategy 上下文构建策略
type ContextStrategy string

const (
	// StrategyRecent 保留最近消息
	StrategyRecent ContextStrategy = "recent"
	// StrategySummary 对旧消息进行摘要
	StrategySummary ContextStrategy = "summary"
	// StrategyImportance 根据重要性保留
	StrategyImportance ContextStrategy = "importance"
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	cfg      *config.Config
	strategy ContextStrategy
}

// ContextBuilderOption 上下文构建器选项
type ContextBuilderOption func(*ContextBuilder)

// WithStrategy 设置策略
func WithStrategy(strategy ContextStrategy) ContextBuilderOption {
	return func(b *ContextBuilder) {
		b.strategy = strategy
	}
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(cfg *config.Config, opts ...ContextBuilderOption) *ContextBuilder {
	b := &ContextBuilder{
		cfg:      cfg,
		strategy: StrategyRecent, // 默认策略
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// BuildMessages 构建 LLM 消息列表
func (b *ContextBuilder) BuildMessages(sess *session.Session) []provider.Message {
	msgs := make([]provider.Message, 0, len(sess.Messages)+1)

	// 添加系统提示
	systemMsg := b.getSystemPrompt()
	if systemMsg != "" {
		msgs = append(msgs, provider.Message{
			Role:    "system",
			Content: systemMsg,
		})
	}

	// 根据策略处理历史消息
	var historyMsgs []session.Message
	switch b.strategy {
	case StrategySummary:
		historyMsgs = b.truncateWithSummary(sess.Messages)
	case StrategyImportance:
		historyMsgs = b.truncateByImportance(sess.Messages)
	default:
		historyMsgs = b.truncateMessages(sess.Messages)
	}

	for _, msg := range historyMsgs {
		providerMsg := b.toProviderMessage(msg)
		msgs = append(msgs, providerMsg)
	}

	return msgs
}

// toProviderMessage 转换消息格式
func (b *ContextBuilder) toProviderMessage(msg session.Message) provider.Message {
	providerMsg := provider.Message{
		Role:    msg.Role,
		Content: msg.Content,
	}

	// 转换工具调用
	if len(msg.ToolCalls) > 0 {
		providerMsg.ToolCalls = make([]provider.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			providerMsg.ToolCalls = append(providerMsg.ToolCalls, provider.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
	}

	// 转换工具结果（如果是 tool 消息且格式是 JSON 数组）
	if msg.Role == "tool" && strings.TrimSpace(msg.Content) != "" {
		var toolResults []map[string]any
		if err := json.Unmarshal([]byte(msg.Content), &toolResults); err == nil {
			// 对于 tool 消息，保持原样，让 provider 处理格式转换
		}
	}

	return providerMsg
}

// truncateMessages 智能截断消息历史以适应上下文限制
func (b *ContextBuilder) truncateMessages(messages []session.Message) []session.Message {
	maxTokens := b.cfg.Session.Context.MaxTokens
	if maxTokens <= 0 {
		return messages // 无限制
	}

	// 简单实现：保留最近 N 条消息
	maxMessages := b.getMaxMessagesForTokens(maxTokens)

	if len(messages) <= maxMessages {
		return messages
	}

	// 保留最新的 maxMessages 条消息
	return messages[len(messages)-maxMessages:]
}

// truncateWithSummary 使用摘要策略截断
func (b *ContextBuilder) truncateWithSummary(messages []session.Message) []session.Message {
	maxTokens := b.cfg.Session.Context.MaxTokens
	if maxTokens <= 0 || len(messages) <= 10 {
		return messages
	}

	maxMessages := b.getMaxMessagesForTokens(maxTokens)
	if len(messages) <= maxMessages {
		return messages
	}

	// 保留最早的系统上下文 + 最近的消息
	keepCount := maxMessages - 2 // 留出 2 条空间给摘要

	// 取前 2 条（可能包含系统上下文介绍）
	result := make([]session.Message, 0, maxMessages)
	if len(messages) > 2 {
		result = append(result, messages[0])
		if messages[1].Role == "user" {
			result = append(result, messages[1])
		}
	}

	// 添加摘要标记
	result = append(result, session.Message{
		Role:    "user",
		Content: "[...中间对话历史已省略...]",
	})

	// 添加最近的消息
	startIdx := len(messages) - keepCount
	if startIdx < 2 {
		startIdx = 2
	}
	result = append(result, messages[startIdx:]...)

	return result
}

// truncateByImportance 根据重要性截断
func (b *ContextBuilder) truncateByImportance(messages []session.Message) []session.Message {
	maxTokens := b.cfg.Session.Context.MaxTokens
	if maxTokens <= 0 || len(messages) <= 10 {
		return messages
	}

	maxMessages := b.getMaxMessagesForTokens(maxTokens)
	if len(messages) <= maxMessages {
		return messages
	}

	// 标记重要消息
	importantIndices := make(map[int]bool)

	// 第一条消息总是重要的
	importantIndices[0] = true

	// 最后几条总是重要的
	for i := len(messages) - 5; i < len(messages); i++ {
		if i >= 0 {
			importantIndices[i] = true
		}
	}

	// 包含工具调用的消息很重要
	for i, msg := range messages {
		if len(msg.ToolCalls) > 0 {
			importantIndices[i] = true
			// 工具响应也很重要
			if i+1 < len(messages) && messages[i+1].Role == "tool" {
				importantIndices[i+1] = true
			}
		}
	}

	// 构建结果
	result := make([]session.Message, 0, maxMessages)
	addedCount := 0
	skipped := false

	for i, msg := range messages {
		if importantIndices[i] || addedCount < 3 {
			// 如果之前跳过了消息，添加一个标记
			if skipped {
				result = append(result, session.Message{
					Role:    "user",
					Content: "[...部分对话已省略...]",
				})
				skipped = false
			}
			result = append(result, msg)
			addedCount++
		} else {
			skipped = true
		}

		if addedCount >= maxMessages {
			break
		}
	}

	return result
}

func (b *ContextBuilder) getMaxMessagesForTokens(maxTokens int) int {
	switch {
	case maxTokens < 4096:
		return 15
	case maxTokens < 8192:
		return 25
	case maxTokens < 16384:
		return 40
	case maxTokens < 32768:
		return 60
	default:
		return 100
	}
}

// getSystemPrompt 获取系统提示
func (b *ContextBuilder) getSystemPrompt() string {
	workspace := config.GetWorkspace()
	return fmt.Sprintf(`You are NeoRay, a helpful AI assistant with access to powerful tools.

Your capabilities include:
- Reading, writing, and editing files in the workspace
- Executing shell commands
- Searching for files and content
- Accessing the web

Current workspace directory: %s

Important guidelines:
1. Always use the appropriate tools for the task at hand
2. When editing files, be precise and use apply_patch for multiple changes
3. Use find_files and grep to explore the codebase before making changes
4. Execute shell commands carefully and review output
5. When asked about code, read the relevant files first
6. Always verify changes after making them
7. Be helpful, thorough, and thoughtful in your responses

When you need to use tools:
- Call one tool at a time, or multiple tools in parallel when appropriate
- Wait for tool results before proceeding
- Use tool results to inform your next steps

Your goal is to help the user accomplish their objectives efficiently and safely.`, workspace)
}
