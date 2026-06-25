package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"neoray/internal/config"
)

// ListExecSessionsTool lists active exec sessions
type ListExecSessionsTool struct {
	cfg            *config.Config
	sessionManager *ExecSessionManager
	sessionKey     string
}

// NewListExecSessionsTool creates a new ListExecSessionsTool
func NewListExecSessionsTool(cfg *config.Config) *ListExecSessionsTool {
	return &ListExecSessionsTool{
		cfg: cfg,
	}
}

// NewListExecSessionsToolWithSessionManager creates a new ListExecSessionsTool with session manager
func NewListExecSessionsToolWithSessionManager(cfg *config.Config, mgr *ExecSessionManager) *ListExecSessionsTool {
	return &ListExecSessionsTool{
		cfg:            cfg,
		sessionManager: mgr,
	}
}

// SetSessionManager sets the session manager
func (t *ListExecSessionsTool) SetSessionManager(mgr *ExecSessionManager) {
	t.sessionManager = mgr
}

// SetSessionKey sets the owner session key
func (t *ListExecSessionsTool) SetSessionKey(key string) {
	t.sessionKey = key
}

// Name returns the tool name
func (t *ListExecSessionsTool) Name() string {
	return "list_exec_sessions"
}

// Description returns the tool description
func (t *ListExecSessionsTool) Description() string {
	return "List active long-running exec sessions, including session_id, cwd, elapsed time, idle time, remaining timeout, and command preview. Use this to recover a session_id after context shifts before polling, writing stdin, or terminating with write_stdin."
}

// Parameters returns the tool parameters
func (t *ListExecSessionsTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{}, []string{})
}

// Execute executes the tool
func (t *ListExecSessionsTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if t.sessionManager == nil {
		res, _ := json.Marshal("No active exec sessions.")
		return res, nil
	}

	sessions := t.sessionManager.List(t.sessionKey)

	if len(sessions) == 0 {
		res, _ := json.Marshal("No active exec sessions.")
		return res, nil
	}

	var lines []string
	for _, info := range sessions {
		command := info.Command
		// Preview - truncate long commands
		if len(command) > 120 {
			command = command[:119] + "…"
		}

		status := "running"
		if info.ReturnCode != nil {
			status = "exited"
		}

		line := fmt.Sprintf(
			"%s | %s | elapsed=%.1fs | idle=%.1fs | remaining=%.1fs | cwd=%s | %s",
			info.SessionID,
			status,
			info.Elapsed,
			info.Idle,
			info.Remaining,
			info.Cwd,
			command,
		)
		lines = append(lines, line)
	}

	result := strings.Join(lines, "\n")
	res, _ := json.Marshal(result)
	return res, nil
}
