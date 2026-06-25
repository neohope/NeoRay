package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"neoray/internal/config"
)

type writeStdinParams struct {
	SessionID      string `json:"session_id"`
	Chars          string `json:"chars,omitempty"`
	CloseStdin     bool   `json:"close_stdin,omitempty"`
	Terminate      bool   `json:"terminate,omitempty"`
	YieldTimeMs    int    `json:"yield_time_ms,omitempty"`
	WaitFor        string `json:"wait_for,omitempty"`
	WaitTimeoutMs  int    `json:"wait_timeout_ms,omitempty"`
	MaxOutputChars int    `json:"max_output_chars,omitempty"`
}

// WriteStdinTool writes to or polls a running exec session
type WriteStdinTool struct {
	cfg            *config.Config
	sessionManager *ExecSessionManager
	sessionKey     string
}

// NewWriteStdinTool creates a new WriteStdinTool
func NewWriteStdinTool(cfg *config.Config) *WriteStdinTool {
	return &WriteStdinTool{
		cfg: cfg,
	}
}

// NewWriteStdinToolWithSessionManager creates a new WriteStdinTool with session manager
func NewWriteStdinToolWithSessionManager(cfg *config.Config, mgr *ExecSessionManager) *WriteStdinTool {
	return &WriteStdinTool{
		cfg:            cfg,
		sessionManager: mgr,
	}
}

// SetSessionManager sets the session manager
func (t *WriteStdinTool) SetSessionManager(mgr *ExecSessionManager) {
	t.sessionManager = mgr
}

// SetSessionKey sets the owner session key
func (t *WriteStdinTool) SetSessionKey(key string) {
	t.sessionKey = key
}

// Name returns the tool name
func (t *WriteStdinTool) Name() string {
	return "write_stdin"
}

// Description returns the tool description
func (t *WriteStdinTool) Description() string {
	return "Interact with a running exec session created by shell with yield_time_ms. Use chars='' to poll without writing, chars to send stdin, close_stdin=true to send EOF, or terminate=true to stop the process. Use wait_for with wait_timeout_ms for dev servers and prompts where you need to wait for expected output."
}

// Parameters returns the tool parameters
func (t *WriteStdinTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{
		"session_id":       StringParam("Session id returned by shell when yield_time_ms is used."),
		"chars":            StringParam("Bytes/text to write to stdin. Omit or pass an empty string to only poll recent output."),
		"close_stdin":      BooleanParam("Close stdin after writing chars. Useful for commands waiting for EOF."),
		"terminate":        BooleanParam("Terminate the running exec session."),
		"yield_time_ms":    NumberParam("Milliseconds to wait before returning recent output (default 1000, max 30000)."),
		"wait_for":         StringParam("Optional text to wait for in output before returning. Useful for interactive commands and dev servers."),
		"wait_timeout_ms":  NumberParam("Maximum milliseconds to wait for wait_for text (default 10000, max 120000)."),
		"max_output_chars": NumberParam("Maximum output characters to return from this poll (default 10000, max 50000)."),
	}, []string{"session_id"})
}

// Execute executes the tool
func (t *WriteStdinTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params writeStdinParams

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if t.sessionManager == nil {
		res, _ := json.Marshal("Error: No session manager available")
		return res, nil
	}

	if params.WaitFor != "" {
		waitTimeoutMs := clampInt(params.WaitTimeoutMs, DefaultWaitForMs, 0, MaxWaitForMs)
		maxOutput := clampInt(params.MaxOutputChars, DefaultMaxOutput, 1000, MaxMaxOutput)
		deadline := time.Now().Add(time.Duration(waitTimeoutMs) * time.Millisecond)

		var aggregateOutput []string
		var poll *SessionPoll
		first := true

		for {
			remainingMs := int(time.Until(deadline).Milliseconds())
			if remainingMs <= 0 {
				break
			}
			stepMs := minInt(500, remainingMs)

			var chars string
			if first {
				chars = params.Chars
			}

			var closeStdin bool
			if first {
				closeStdin = params.CloseStdin
			}

			var terminate bool
			if first {
				terminate = params.Terminate
			}

			var err error
			poll, err = t.sessionManager.Write(
				params.SessionID,
				chars,
				closeStdin,
				terminate,
				stepMs,
				maxOutput,
				t.sessionKey,
			)
			if err != nil {
				res, _ := json.Marshal(fmt.Sprintf("Error: %s", err.Error()))
				return res, nil
			}
			first = false

			if poll.Output != "" {
				aggregateOutput = append(aggregateOutput, poll.Output)
				joined := strings.Join(aggregateOutput, "")
				if strings.Contains(joined, params.WaitFor) {
					poll.Output = joined
					result := FormatSessionPoll(params.SessionID, poll)
					res, _ := json.Marshal(result)
					return res, nil
				}
			}

			if poll.Done {
				break
			}
		}

		// Timed out or done - return aggregate
		if poll != nil {
			poll.Output = strings.Join(aggregateOutput, "")
			result := FormatSessionPoll(params.SessionID, poll)
			if !strings.Contains(poll.Output, params.WaitFor) {
				result += fmt.Sprintf("\nWait target not observed: %q", params.WaitFor)
			}
			res, _ := json.Marshal(result)
			return res, nil
		}

		res, _ := json.Marshal("Error: No poll result")
		return res, nil
	}

	yieldMs := clampInt(params.YieldTimeMs, DefaultYieldMs, 0, MaxYieldMs)
	maxOutput := clampInt(params.MaxOutputChars, DefaultMaxOutput, 1000, MaxMaxOutput)

	poll, err := t.sessionManager.Write(
		params.SessionID,
		params.Chars,
		params.CloseStdin,
		params.Terminate,
		yieldMs,
		maxOutput,
		t.sessionKey,
	)
	if err != nil {
		res, _ := json.Marshal(fmt.Sprintf("Error: %s", err.Error()))
		return res, nil
	}

	result := FormatSessionPoll(params.SessionID, poll)
	res, _ := json.Marshal(result)
	return res, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
