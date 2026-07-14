package agent

import (
	"encoding/json"
	"sync"
	"time"
)

// TraceStep 追踪步骤
type TraceStep struct {
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"` // "llm_call", "tool_call", "error", "info"
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
}

// TraceSession 会话追踪
type TraceSession struct {
	SessionID string      `json:"session_id"`
	Steps     []TraceStep `json:"steps"`
	StartTime time.Time   `json:"start_time"`
	mu        sync.Mutex
}

// NewTraceSession 创建会话追踪
func NewTraceSession(sessionID string) *TraceSession {
	return &TraceSession{
		SessionID: sessionID,
		Steps:     make([]TraceStep, 0, 10),
		StartTime: time.Now(),
	}
}

// AddStep 添加步骤
func (ts *TraceSession) AddStep(step TraceStep) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.Steps = append(ts.Steps, step)
}

// AddLLMCall 添加 LLM 调用追踪
func (ts *TraceSession) AddLLMCall(iteration int, inputTokens, outputTokens int, duration time.Duration) {
	ts.AddStep(TraceStep{
		Timestamp:   time.Now(),
		Type:        "llm_call",
		Description: "LLM 调用",
		Details: map[string]interface{}{
			"iteration":     iteration,
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
		Duration: duration,
	})
}

// AddToolCall 添加工具调用追踪
func (ts *TraceSession) AddToolCall(toolName, toolID string, isError bool, duration time.Duration) {
	ts.AddStep(TraceStep{
		Timestamp:   time.Now(),
		Type:        "tool_call",
		Description: "Tool call: " + toolName,
		Details: map[string]interface{}{
			"tool_name": toolName,
			"tool_id":   toolID,
			"is_error":  isError,
		},
		Duration: duration,
	})
}

// AddError 添加错误追踪
func (ts *TraceSession) AddError(err error, context string) {
	ts.AddStep(TraceStep{
		Timestamp:   time.Now(),
		Type:        "error",
		Description: context + ": " + err.Error(),
	})
}

// AddInfo 添加信息追踪
func (ts *TraceSession) AddInfo(message string, details map[string]interface{}) {
	ts.AddStep(TraceStep{
		Timestamp:   time.Now(),
		Type:        "info",
		Description: message,
		Details:     details,
	})
}

// ToJSON 导出为 JSON
func (ts *TraceSession) ToJSON() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	data, _ := json.MarshalIndent(ts, "", "  ")
	return string(data)
}

// GetTotalDuration 获取总耗时
func (ts *TraceSession) GetTotalDuration() time.Duration {
	return time.Since(ts.StartTime)
}

// TraceManager 追踪管理器
type TraceManager struct {
	sessions map[string]*TraceSession
	mu       sync.RWMutex
	enabled  bool
}

// NewTraceManager 创建追踪管理器
func NewTraceManager(enabled bool) *TraceManager {
	return &TraceManager{
		sessions: make(map[string]*TraceSession),
		enabled:  enabled,
	}
}

// IsEnabled 检查是否启用
func (tm *TraceManager) IsEnabled() bool {
	return tm.enabled
}

// GetOrCreateSession 获取或创建追踪会话
func (tm *TraceManager) GetOrCreateSession(sessionID string) *TraceSession {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if ts, ok := tm.sessions[sessionID]; ok {
		return ts
	}
	ts := NewTraceSession(sessionID)
	tm.sessions[sessionID] = ts
	return ts
}

// RemoveSession 移除会话
func (tm *TraceManager) RemoveSession(sessionID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.sessions, sessionID)
}

// GetSession 获取会话（只读）
func (tm *TraceManager) GetSession(sessionID string) (*TraceSession, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	ts, ok := tm.sessions[sessionID]
	return ts, ok
}
