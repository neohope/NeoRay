package agent

import (
	"context"

	"neoray/internal/session"
)

// AgentInterface 定义 Agent 的公共接口。
// 所有外部调用方应使用此接口而非具体类型。
type AgentInterface interface {
	// Chat 发送聊天消息，返回完整响应。
	Chat(ctx context.Context, sess *session.Session, userInput string) (*ChatResult, error)

	// ChatStream 发送聊天消息，返回流式响应通道。
	ChatStream(ctx context.Context, sess *session.Session, userInput string) (<-chan StreamChunk, error)

	// Start 启动 Agent，注册消息总线处理。
	Start() error
}
