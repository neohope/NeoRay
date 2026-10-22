package session

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// Session 会话实体
type Session struct {
	ID        string         `json:"id"`
	ChannelID string         `json:"channel_id"` // 频道标识符
	UserID    string         `json:"user_id"`    // 用户标识符
	Title     string         `json:"title"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Messages  []Message      `json:"messages"`
	Metadata  map[string]any `json:"metadata,omitempty"`

	mu sync.RWMutex `json:"-"`
}

// Lock acquires the session write lock.
func (s *Session) Lock() { s.mu.Lock() }

// Unlock releases the session write lock.
func (s *Session) Unlock() { s.mu.Unlock() }

// RLock acquires the session read lock.
func (s *Session) RLock() { s.mu.RLock() }

// RUnlock releases the session read lock.
func (s *Session) RUnlock() { s.mu.RUnlock() }

// Message 消息实体
type Message struct {
	ID          string         `json:"id"`
	ChannelID   string         `json:"channel_id"` // 频道标识符
	UserID      string         `json:"user_id"`    // 用户标识符
	SessionID   string         `json:"session_id"` // 会话标识符
	Role        string         `json:"role"`       // user, assistant, system, tool
	Content     string         `json:"content"`
	Timestamp   time.Time      `json:"timestamp"`
	ToolCalls   []ToolCall     `json:"tool_calls,omitempty"`
	ToolResults []ToolResult   `json:"tool_results,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolResult 工具结果
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// NewSession 创建新会话
func NewSession(channelID, userID string) *Session {
	now := time.Now()
	return &Session{
		ID:        generateID(),
		ChannelID: channelID,
		UserID:    userID,
		Title:     "New Session",
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  make([]Message, 0),
		Metadata:  make(map[string]any),
	}
}

// NewUserMessage 创建用户消息
func NewUserMessage(channelID, userID, sessionID, content string) Message {
	return Message{
		ID:        generateID(),
		ChannelID: channelID,
		UserID:    userID,
		SessionID: sessionID,
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// NewAssistantMessage 创建助手消息
func NewAssistantMessage(channelID, userID, sessionID, content string) Message {
	return Message{
		ID:        generateID(),
		ChannelID: channelID,
		UserID:    userID,
		SessionID: sessionID,
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// NewSystemMessage 创建系统消息
func NewSystemMessage(channelID, userID, sessionID, content string) Message {
	return Message{
		ID:        generateID(),
		ChannelID: channelID,
		UserID:    userID,
		SessionID: sessionID,
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// NewToolMessage 创建工具消息
func NewToolMessage(channelID, userID, sessionID, content string) Message {
	return Message{
		ID:        generateID(),
		ChannelID: channelID,
		UserID:    userID,
		SessionID: sessionID,
		Role:      "tool",
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// AddMessage 添加消息（线程安全）
func (s *Session) AddMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保消息的频道和用户与会话一致
	msg.ChannelID = s.ChannelID
	msg.UserID = s.UserID
	msg.SessionID = s.ID

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()

	// 自动更新标题（如果是第一条用户消息）
	if s.Title == "New Session" && msg.Role == "user" && len(msg.Content) > 0 {
		s.Title = truncateString(msg.Content, 30)
	}
}

// LastMessage 获取最后一条消息（线程安全）
func (s *Session) LastMessage() *Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Messages) == 0 {
		return nil
	}
	// 返回副本而非指针，避免外部修改内部数据
	last := s.Messages[len(s.Messages)-1]
	return &last
}

// LastAssistantContentSince 返回下标 index（含）之后最后一条非空 assistant 消息的内容。
// 用于"只在本轮新增的消息范围内"提取回复，避免误取历史旧答案（线程安全）。
func (s *Session) LastAssistantContentSince(index int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if index < 0 {
		index = 0
	}
	for i := len(s.Messages) - 1; i >= index; i-- {
		msg := s.Messages[i]
		if msg.Role == "assistant" && strings.TrimSpace(msg.Content) != "" {
			return msg.Content
		}
	}
	return ""
}

// MetadataSnapshot 返回 metadata 的浅拷贝（在读取锁内完成）。
// 调用方拿到的 map 可以安全遍历，不会与并发写入冲突。
func (s *Session) MetadataSnapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Metadata == nil {
		return nil
	}
	out := make(map[string]any, len(s.Metadata))
	for k, v := range s.Metadata {
		out[k] = v
	}
	return out
}

// SetMetadataValue 设置一个 metadata 键（写时复制，锁内完成）
func (s *Session) SetMetadataValue(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta := make(map[string]any, len(s.Metadata)+1)
	for k, v := range s.Metadata {
		meta[k] = v
	}
	meta[key] = value
	s.Metadata = meta
}

// RemoveLastUserMessage 移除最后一条 user 消息（线程安全）。
// 失败路径要用它而不是 RemoveLastMessage：第 2 轮及以后最后一条是 tool 结果，
// 删掉它会留下「assistant(tool_calls) 后面没有 tool 结果」的非法历史，
// 之后每轮请求都被上游 400。
func (s *Session) RemoveLastUserMessage() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == "user" {
			s.Messages = append(s.Messages[:i], s.Messages[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveLastMessage 移除最后一条消息（线程安全）
func (s *Session) RemoveLastMessage() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Messages) > 0 {
		s.Messages = s.Messages[:len(s.Messages)-1]
	}
}

// Clear 清空会话消息（线程安全）
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = make([]Message, 0)
	s.Title = "New Session"
	s.UpdatedAt = time.Now()
}

// DeepCopy 返回 Session 的深拷贝，防止调用方绕过锁直接修改内部状态。
// 注意：不能用 cp := *s 复制整个 struct，因为 sync.RWMutex 在首次使用后不能复制。
// 复制已锁定的 mutex 会导致拷贝的 mutex 也处于锁定状态，造成死锁。
func (s *Session) DeepCopy() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := &Session{
		ID:        s.ID,
		ChannelID: s.ChannelID,
		UserID:    s.UserID,
		Title:     s.Title,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		// mu 留零值，不复制原始 mutex
	}

	// 注意：只 copy slice 头是浅拷贝 —— Message 里的 ToolCalls / ToolResults /
	// Metadata 仍与会话内部共享。FileStore.Get/Save 依赖 DeepCopy 做并发隔离，
	// 任何一方修改嵌套 map 都会触发 concurrent map read/write（进程 panic）。
	cp.Messages = make([]Message, len(s.Messages))
	for i, msg := range s.Messages {
		cp.Messages[i] = copyMessage(msg)
	}

	if s.Metadata != nil {
		cp.Metadata = deepCopyValue(s.Metadata, 0).(map[string]any)
	}

	return cp
}

// copyMessage 复制单条消息，包含嵌套的 slice / map
func copyMessage(msg Message) Message {
	out := msg
	if msg.ToolCalls != nil {
		out.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
		copy(out.ToolCalls, msg.ToolCalls)
	}
	if msg.ToolResults != nil {
		out.ToolResults = make([]ToolResult, len(msg.ToolResults))
		copy(out.ToolResults, msg.ToolResults)
	}
	if msg.Metadata != nil {
		out.Metadata = deepCopyValue(msg.Metadata, 0).(map[string]any)
	}
	return out
}

// maxDeepCopyDepth 防止异常数据结构（自引用）导致无限递归
const maxDeepCopyDepth = 16

// deepCopyValue 递归复制 map[string]any / []any / []string 等常见容器。
// 标量（string/number/bool/time.Time 等）按值复制即可。
func deepCopyValue(v any, depth int) any {
	if depth > maxDeepCopyDepth {
		return v
	}
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = deepCopyValue(item, depth+1)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(val))
		for k, item := range val {
			out[k] = item
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = deepCopyValue(item, depth+1)
		}
		return out
	case []string:
		out := make([]string, len(val))
		copy(out, val)
		return out
	case []int:
		out := make([]int, len(val))
		copy(out, val)
		return out
	case []float64:
		out := make([]float64, len(val))
		copy(out, val)
		return out
	case []bool:
		out := make([]bool, len(val))
		copy(out, val)
		return out
	default:
		return v
	}
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
