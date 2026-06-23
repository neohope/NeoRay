package subagent

import (
	"sync"
	"time"

	"neoray/internal/provider"
)

// SubagentPhase 子代理执行阶段
type SubagentPhase string

const (
	// PhaseInitializing 初始化阶段
	PhaseInitializing SubagentPhase = "initializing"
	// PhaseAwaitingTools 等待工具调用
	PhaseAwaitingTools SubagentPhase = "awaiting_tools"
	// PhaseToolsCompleted 工具调用完成
	PhaseToolsCompleted SubagentPhase = "tools_completed"
	// PhaseFinalResponse 生成最终响应
	PhaseFinalResponse SubagentPhase = "final_response"
	// PhaseDone 完成
	PhaseDone SubagentPhase = "done"
	// PhaseError 错误
	PhaseError SubagentPhase = "error"
)

// ToolEvent 工具调用事件
type ToolEvent struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // "ok" 或 "error"
	Detail    string `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

// SubagentStatus 子代理实时状态
type SubagentStatus struct {
	TaskID          string            `json:"task_id"`
	Label           string            `json:"label"`
	TaskDescription string            `json:"task_description"`
	StartedAt       time.Time         `json:"started_at"`
	Phase           SubagentPhase    `json:"phase"`
	Iteration       int               `json:"iteration"`
	ToolEvents      []ToolEvent       `json:"tool_events"`
	Usage           *provider.Usage    `json:"usage"` // token usage
	StopReason      string            `json:"stop_reason,omitempty"`
	Error           string            `json:"error,omitempty"`

	mu sync.RWMutex
}

// NewSubagentStatus 创建新的子代理状态
func NewSubagentStatus(taskID, label, taskDescription string) *SubagentStatus {
	return &SubagentStatus{
		TaskID:          taskID,
		Label:           label,
		TaskDescription: taskDescription,
		StartedAt:       time.Now(),
		Phase:           PhaseInitializing,
		Iteration:       0,
		ToolEvents:      []ToolEvent{},
	}
}

// SetPhase 设置执行阶段
func (s *SubagentStatus) SetPhase(phase SubagentPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Phase = phase
}

// GetPhase 获取当前阶段
func (s *SubagentStatus) GetPhase() SubagentPhase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Phase
}

// SetIteration 设置迭代次数
func (s *SubagentStatus) SetIteration(iteration int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Iteration = iteration
}

// AddToolEvent 添加工具事件
func (s *SubagentStatus) AddToolEvent(name, status, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolEvents = append(s.ToolEvents, ToolEvent{
		Name:      name,
		Status:    status,
		Detail:    detail,
		Timestamp: time.Now(),
	})
}

// SetUsage 设置 token 使用量
func (s *SubagentStatus) SetUsage(usage *provider.Usage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Usage = usage
}

// SetError 设置错误信息
func (s *SubagentStatus) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Error = err
	s.Phase = PhaseError
}

// SetStopReason 设置停止原因
func (s *SubagentStatus) SetStopReason(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StopReason = reason
}

// GetToolEvents 获取所有工具事件
func (s *SubagentStatus) GetToolEvents() []ToolEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]ToolEvent, len(s.ToolEvents))
	copy(events, s.ToolEvents)
	return events
}

// IsRunning 检查是否正在运行
func (s *SubagentStatus) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Phase != PhaseDone && s.Phase != PhaseError
}

// Duration 获取运行时长
func (s *SubagentStatus) Duration() time.Duration {
	return time.Since(s.StartedAt)
}
