package bus

import (
	"context"
	"time"
)

// MessageType 定义消息类型
type MessageType string

const (
	// ==================== 内部总线类型 ====================
	// MessageTypeUser 用户消息（入站）
	MessageTypeUser MessageType = "user"
	// MessageTypeSystem 系统消息（入站）
	MessageTypeSystem MessageType = "system"
	// MessageTypeAssistant Assistant 回复（出站）
	MessageTypeAssistant MessageType = "assistant"
	// MessageTypeToolCall 工具调用（出站/入站）
	MessageTypeToolCall MessageType = "tool_call"
	// MessageTypeToolResult 工具结果（入站/出站）
	MessageTypeToolResult MessageType = "tool_result"
	// MessageTypeDelta 流式增量（出站）
	MessageTypeDelta MessageType = "delta"
	// MessageTypeError 错误消息（出站）
	MessageTypeError MessageType = "error"

	// ==================== WebSocket 协议类型 ====================
	// MessageTypeChatStart 聊天开始
	MessageTypeChatStart MessageType = "chat_start"
	// MessageTypeChatChunk 聊天内容块
	MessageTypeChatChunk MessageType = "chat_chunk"
	// MessageTypeChatEnd 聊天结束
	MessageTypeChatEnd MessageType = "chat_end"
	// MessageTypeToolCallStart 工具调用开始
	MessageTypeToolCallStart MessageType = "tool_call_start"
	// MessageTypeToolCallResult 工具调用结果
	MessageTypeToolCallResult MessageType = "tool_call_result"
	// MessageTypeSessionCreated 会话已创建
	MessageTypeSessionCreated MessageType = "session_created"
	// MessageTypeSessionJoined 已加入会话
	MessageTypeSessionJoined MessageType = "session_joined"
	// MessageTypeSessionList 会话列表
	MessageTypeSessionList MessageType = "session_list"
	// MessageTypeProgress 进度/系统状态消息
	MessageTypeProgress MessageType = "progress"
)

// InboundMessage 入站消息（从频道到总线）
type InboundMessage struct {
	// ID 消息唯一 ID
	ID string
	// Type 消息类型
	Type MessageType
	// ChannelID 来源频道 ID
	ChannelID string
	// ChatID 聊天 ID（频道内的会话标识）
	ChatID string
	// UserID 用户 ID
	UserID string
	// Content 消息内容
	Content string
	// Media 媒体文件列表
	Media []string
	// Metadata 元数据
	Metadata map[string]interface{}
	// Timestamp 时间戳
	Timestamp time.Time
	// Context 上下文（用于传递取消等）
	Context context.Context
}

// OutboundMessage 出站消息（从总线到频道）
type OutboundMessage struct {
	// ID 消息唯一 ID
	ID string
	// Type 消息类型
	Type MessageType
	// ChannelID 目标频道 ID
	ChannelID string
	// ChatID 目标聊天 ID
	ChatID string
	// SessionID 会话 ID（可选）
	SessionID string
	// Content 消息内容
	Content string
	// Media 媒体文件列表
	Media []string
	// Metadata 元数据
	Metadata map[string]interface{}
	// Timestamp 时间戳
	Timestamp time.Time
}

// NewInboundMessage 创建入站消息
func NewInboundMessage(channelID, chatID, userID, content string) *InboundMessage {
	return &InboundMessage{
		ID:         generateID(),
		Type:       MessageTypeUser,
		ChannelID:  channelID,
		ChatID:     chatID,
		UserID:     userID,
		Content:    content,
		Media:      []string{},
		Metadata:   make(map[string]interface{}),
		Timestamp:  time.Now(),
		Context:    context.Background(),
	}
}

// NewOutboundMessage 创建出站消息
func NewOutboundMessage(channelID, chatID, content string) *OutboundMessage {
	return &OutboundMessage{
		ID:         generateID(),
		Type:       MessageTypeAssistant,
		ChannelID:  channelID,
		ChatID:     chatID,
		Content:    content,
		Media:      []string{},
		Metadata:   make(map[string]interface{}),
		Timestamp:  time.Now(),
	}
}

// WithContext 添加上下文
func (m *InboundMessage) WithContext(ctx context.Context) *InboundMessage {
	m.Context = ctx
	return m
}

// WithMetadata 添加元数据
func (m *InboundMessage) WithMetadata(key string, value interface{}) *InboundMessage {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
	return m
}

// WithMetadata 添加元数据
func (m *OutboundMessage) WithMetadata(key string, value interface{}) *OutboundMessage {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
	return m
}

// WithSessionID 设置会话 ID
func (m *OutboundMessage) WithSessionID(sessionID string) *OutboundMessage {
	m.SessionID = sessionID
	return m
}

// WithType 设置消息类型
func (m *OutboundMessage) WithType(msgType MessageType) *OutboundMessage {
	m.Type = msgType
	return m
}

// WithMedia 设置媒体文件
func (m *OutboundMessage) WithMedia(media []string) *OutboundMessage {
	m.Media = media
	return m
}

// generateID 生成简单的消息 ID
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}

// SessionKey 获取会话键
func (m *InboundMessage) SessionKey() string {
	if m.ChannelID != "" && m.ChatID != "" {
		return m.ChannelID + ":" + m.ChatID
	}
	return m.ID
}

// SessionKeyOverride 获取覆盖的会话键
func (m *InboundMessage) SessionKeyOverride() string {
	if m.Metadata == nil {
		return ""
	}
	if override, ok := m.Metadata["session_key_override"].(string); ok {
		return override
	}
	return ""
}

// WithSessionKeyOverride 设置覆盖的会话键
func (m *InboundMessage) WithSessionKeyOverride(sessionKey string) *InboundMessage {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata["session_key_override"] = sessionKey
	return m
}
