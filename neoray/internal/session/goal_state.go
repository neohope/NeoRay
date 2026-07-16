package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// GoalStateKey goal state 在 session metadata 中的 key
const GoalStateKey = "goal_state"

// GoalStatus goal 状态
type GoalStatus string

const (
	GoalStatusActive   GoalStatus = "active"
	GoalStatusComplete GoalStatus = "completed"
)

// GoalState 长期目标状态
type GoalState struct {
	Status    GoalStatus `json:"status"`
	Objective string     `json:"objective"`
	UISummary string     `json:"ui_summary,omitempty"`
	StartedAt string     `json:"started_at,omitempty"`
	EndedAt   string     `json:"ended_at,omitempty"`
	Recap     string     `json:"recap,omitempty"`
}

// GoalManager Goal 管理器
type GoalManager struct {
	mu         sync.RWMutex
	sessionMgr *Manager
	bus        any
}

// NewGoalManager 创建 Goal 管理器
func NewGoalManager(sessionMgr *Manager, bus any) *GoalManager {
	return &GoalManager{
		sessionMgr: sessionMgr,
		bus:        bus,
	}
}

// GetGoalState 获取当前的 goal state
func (m *GoalManager) GetGoalState(session *Session) (*GoalState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getGoalStateLocked(session)
}

// getGoalStateLocked 获取当前的 goal state（调用者必须持有 m.mu 读锁或写锁）
func (m *GoalManager) getGoalStateLocked(session *Session) (*GoalState, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}

	session.RLock()
	meta := session.Metadata
	session.RUnlock()
	if meta == nil {
		return nil, nil
	}

	if val, ok := meta[GoalStateKey]; ok {
		if stateMap, ok := val.(map[string]any); ok {
			state := &GoalState{}
			if status, ok := stateMap["status"].(string); ok {
				state.Status = GoalStatus(status)
			}
			if objective, ok := stateMap["objective"].(string); ok {
				state.Objective = objective
			}
			if uiSummary, ok := stateMap["ui_summary"].(string); ok {
				state.UISummary = uiSummary
			}
			if startedAt, ok := stateMap["started_at"].(string); ok {
				state.StartedAt = startedAt
			}
			if endedAt, ok := stateMap["ended_at"].(string); ok {
				state.EndedAt = endedAt
			}
			if recap, ok := stateMap["recap"].(string); ok {
				state.Recap = recap
			}
			return state, nil
		}
		if data, err := json.Marshal(val); err == nil {
			var state GoalState
			if json.Unmarshal(data, &state) == nil {
				return &state, nil
			}
		}
	}

	return nil, nil
}

// SetGoalState 设置 goal state
func (m *GoalManager) SetGoalState(session *Session, state *GoalState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.setGoalStateLocked(session, state)
}

// setGoalStateLocked 设置 goal state（调用者必须持有 m.mu 写锁）
func (m *GoalManager) setGoalStateLocked(session *Session, state *GoalState) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	if session.Metadata == nil {
		session.Metadata = make(map[string]any)
	}

	stateMap := map[string]any{
		"status":     string(state.Status),
		"objective":  state.Objective,
		"ui_summary": state.UISummary,
		"started_at": state.StartedAt,
		"ended_at":   state.EndedAt,
		"recap":      state.Recap,
	}
	session.Metadata[GoalStateKey] = stateMap

	if m.sessionMgr != nil {
		if err := m.sessionMgr.SaveSession(session); err != nil {
			return err
		}
	}

	return nil
}

// StartGoal 开始一个新的 goal
func (m *GoalManager) StartGoal(session *Session, objective string, uiSummary string) (*GoalState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existingState, _ := m.getGoalStateLocked(session)
	if existingState != nil && existingState.Status == GoalStatusActive {
		return nil, fmt.Errorf("a goal is already active: %s", existingState.Objective)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	state := &GoalState{
		Status:    GoalStatusActive,
		Objective: objective,
		UISummary: uiSummary,
		StartedAt: now,
	}

	if err := m.setGoalStateLocked(session, state); err != nil {
		return nil, err
	}

	return state, nil
}

// CompleteGoal 完成当前的 goal
func (m *GoalManager) CompleteGoal(session *Session, recap string) (*GoalState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, _ := m.getGoalStateLocked(session)
	if state == nil || state.Status != GoalStatusActive {
		return nil, fmt.Errorf("no active goal to complete")
	}

	state.Status = GoalStatusComplete
	state.EndedAt = time.Now().UTC().Format(time.RFC3339)
	state.Recap = recap

	if err := m.setGoalStateLocked(session, state); err != nil {
		return nil, err
	}

	return state, nil
}

// GetGoalStateRuntimeLines 获取运行时上下文中的 goal state 行
func GetGoalStateRuntimeLines(state *GoalState) []string {
	if state == nil {
		return nil
	}

	var lines []string
	if state.Status == GoalStatusActive {
		lines = append(lines, fmt.Sprintf("Goal (active): %s", state.Objective))
		if state.UISummary != "" {
			lines = append(lines, fmt.Sprintf("  Summary: %s", state.UISummary))
		}
		if state.StartedAt != "" {
			lines = append(lines, fmt.Sprintf("  Started: %s", state.StartedAt))
		}
	} else if state.Status == GoalStatusComplete {
		lines = append(lines, fmt.Sprintf("Goal (completed): %s", state.Objective))
		if state.Recap != "" {
			lines = append(lines, fmt.Sprintf("  Recap: %s", state.Recap))
		}
	}

	return lines
}