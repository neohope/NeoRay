package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"neoray/internal/config"
	"neoray/internal/provider"
	"neoray/internal/session"
)

// ContextBuilder 上下文构建器
type ContextBuilder struct {
	cfg *config.Config
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(cfg *config.Config) *ContextBuilder {
	return &ContextBuilder{cfg: cfg}
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

	// 添加历史消息（带智能截断）
	historyMsgs := b.truncateMessages(sess.Messages)
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
	maxMessages := 50 // 默认值
	if maxTokens < 4096 {
		maxMessages = 20
	} else if maxTokens < 8192 {
		maxMessages = 30
	}

	if len(messages) <= maxMessages {
		return messages
	}

	// 保留最新的 maxMessages 条消息
	return messages[len(messages)-maxMessages:]
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
