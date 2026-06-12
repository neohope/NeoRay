package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Session 会话实体
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Message 消息实体
type Message struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"` // user, assistant, system, tool
	Content   string         `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
	ToolCalls []ToolCall     `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolResult 工具结果
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// NewSession 创建新会话
func NewSession() *Session {
	now := time.Now()
	return &Session{
		ID:        generateID(),
		Title:     "New Session",
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  make([]Message, 0),
		Metadata:  make(map[string]any),
	}
}

// NewUserMessage 创建用户消息
func NewUserMessage(content string) Message {
	return Message{
		ID:        generateID(),
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// NewAssistantMessage 创建助手消息
func NewAssistantMessage(content string) Message {
	return Message{
		ID:        generateID(),
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// NewSystemMessage 创建系统消息
func NewSystemMessage(content string) Message {
	return Message{
		ID:        generateID(),
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// NewToolMessage 创建工具消息
func NewToolMessage(content string) Message {
	return Message{
		ID:        generateID(),
		Role:      "tool",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// AddMessage 添加消息
func (s *Session) AddMessage(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()

	// 自动更新标题（如果是第一条用户消息）
	if s.Title == "New Session" && msg.Role == "user" && len(msg.Content) > 0 {
		s.Title = truncateString(msg.Content, 30)
	}
}

// LastMessage 获取最后一条消息
func (s *Session) LastMessage() *Message {
	if len(s.Messages) == 0 {
		return nil
	}
	return &s.Messages[len(s.Messages)-1]
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
