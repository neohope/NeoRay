package agent

import (
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
	msgs := make([]provider.Message, 0, len(sess.Messages))

	// 添加系统提示
	if b.cfg.Session.Context.MaxTokens > 0 {
		systemMsg := b.getSystemPrompt()
		msgs = append(msgs, provider.Message{
			Role:    "system",
			Content: systemMsg,
		})
	}

	// 添加历史消息
	for _, msg := range sess.Messages {
		providerMsg := provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// 转换工具调用
		if len(msg.ToolCalls) > 0 {
			providerMsg.ToolCalls = make([]provider.ToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				providerMsg.ToolCalls = append(providerMsg.ToolCalls, provider.ToolCall{
					ID:       tc.ID,
					Name:     tc.Name,
					Arguments: tc.Arguments,
				})
			}
		}

		msgs = append(msgs, providerMsg)
	}

	return msgs
}

// getSystemPrompt 获取系统提示
func (b *ContextBuilder) getSystemPrompt() string {
	return `You are a helpful AI assistant.`
}
