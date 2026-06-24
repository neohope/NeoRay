package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"neoray/internal/session"
)

// ======================================
// LongTaskTool
// ======================================

// LongTaskTool long_task 工具
type LongTaskTool struct {
	goalMgr    *session.GoalManager
	getSession func() *session.Session
}

// LongTaskArgs long_task 参数
type LongTaskArgs struct {
	Goal      string `json:"goal"`
	UISummary string `json:"ui_summary,omitempty"`
}

// NewLongTaskTool 创建 long_task 工具
func NewLongTaskTool(goalMgr *session.GoalManager, getSession func() *session.Session) *LongTaskTool {
	return &LongTaskTool{
		goalMgr:    goalMgr,
		getSession: getSession,
	}
}

// Name 返回工具名称
func (t *LongTaskTool) Name() string {
	return "long_task"
}

// Description 返回工具描述
func (t *LongTaskTool) Description() string {
	return "Mark this thread as a sustained long-running task. " +
		"First read the built-in 'long-goal' skill, especially its Start fast section; then call this as soon as the user's intent is clear. " +
		"Write a good idempotent goal, but do not delay the tool call with long planning, research, or execution-detail thinking. " +
		"The active goal is mirrored in Runtime Context each turn. Use normal tools until done, then call complete_goal when the objective is satisfied, canceled, or replaced."
}

// Parameters 返回参数定义
func (t *LongTaskTool) Parameters() json.RawMessage {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"goal": map[string]interface{}{
				"type":        "string",
				"description": "Sustained objective for this chat thread. First read the built-in 'long-goal' skill, especially its Start fast section, then call this promptly once the user's intent is clear. The goal must still be idempotent, self-contained, bounded, and explicit about done-ness; do not delay this tool call to over-plan, research, or decide execution details.",
				"minLength":   1,
			},
			"ui_summary": map[string]interface{}{
				"type":        "string",
				"description": "Optional short label for session lists / logs (max 120 chars).",
				"maxLength":   120,
			},
		},
		"required": []string{"goal"},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return data
}

// Execute 执行工具
func (t *LongTaskTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input LongTaskArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if input.Goal == "" {
		return json.Marshal("Error: goal is required")
	}

	sess := t.getSession()
	if sess == nil {
		return json.Marshal("Error: long_task requires an active chat session (missing session context)")
	}

	// 检查是否已有 active goal
	existingState, _ := t.goalMgr.GetGoalState(sess)
	if existingState != nil && existingState.Status == session.GoalStatusActive {
		return json.Marshal(fmt.Sprintf("Error: a sustained goal is already active. Use complete_goal when finished, or ask the user before replacing it. Current goal: %s", existingState.Objective))
	}

	// 截断 ui_summary
	uiSummary := input.UISummary
	if len(uiSummary) > 120 {
		uiSummary = uiSummary[:120]
	}

	// 创建 goal
	state, err := t.goalMgr.StartGoal(sess, input.Goal, uiSummary)
	if err != nil {
		return json.Marshal(fmt.Sprintf("Error: %s", err))
	}

	result := "Goal recorded. Keep working toward the objective using ordinary tools. When fully done (verified against what was asked), call complete_goal with a short recap."
	if state.UISummary != "" {
		result += fmt.Sprintf("\nSummary line: %s", state.UISummary)
	}

	return json.Marshal(result)
}

// ======================================
// CompleteGoalTool
// ======================================

// CompleteGoalTool complete_goal 工具
type CompleteGoalTool struct {
	goalMgr    *session.GoalManager
	getSession func() *session.Session
}

// CompleteGoalArgs complete_goal 参数
type CompleteGoalArgs struct {
	Recap string `json:"recap,omitempty"`
}

// NewCompleteGoalTool 创建 complete_goal 工具
func NewCompleteGoalTool(goalMgr *session.GoalManager, getSession func() *session.Session) *CompleteGoalTool {
	return &CompleteGoalTool{
		goalMgr:    goalMgr,
		getSession: getSession,
	}
}

// Name 返回工具名称
func (t *CompleteGoalTool) Name() string {
	return "complete_goal"
}

// Description 返回工具描述
func (t *CompleteGoalTool) Description() string {
	return "End bookkeeping for the active sustained goal. Use when the objective is fully achieved and verified — recap what was delivered. Also call when the user cancels, redirects, or replaces the goal: recap must reflect what actually happened (not necessarily success). If no goal is active, the tool reports that and leaves metadata unchanged."
}

// Parameters 返回参数定义
func (t *CompleteGoalTool) Parameters() json.RawMessage {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"recap": map[string]interface{}{
				"type":        "string",
				"description": "Brief recap for the user (plain text). When goal succeeded, confirm outcomes; if user cancelled, pivoted, or replaced, say so honestly.",
				"maxLength":   8000,
			},
		},
		"required": []string{},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return data
}

// Execute 执行工具
func (t *CompleteGoalTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input CompleteGoalArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	sess := t.getSession()
	if sess == nil {
		return json.Marshal("Error: complete_goal requires an active chat session")
	}

	// 检查是否有 active goal
	state, _ := t.goalMgr.GetGoalState(sess)
	if state == nil || state.Status != session.GoalStatusActive {
		return json.Marshal("No active goal to complete")
	}

	// 完成 goal
	endedAt := time.Now().UTC().Format(time.RFC3339)
	_, err := t.goalMgr.CompleteGoal(sess, input.Recap)
	if err != nil {
		return json.Marshal(fmt.Sprintf("Error: %s", err))
	}

	if input.Recap != "" {
		return json.Marshal(fmt.Sprintf("Goal marked complete (%s). Recap:\n%s", endedAt, input.Recap))
	}
	return json.Marshal(fmt.Sprintf("Goal marked complete (%s).", endedAt))
}