package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/security"
)

// ShellTool Shell 执行工具
type ShellTool struct {
	cfg                *config.Config
	workspace          string
	timeout            time.Duration
	sessionManager     *ExecSessionManager
	sessionKey         string
}

// NewShellTool 创建 Shell 工具
func NewShellTool(cfg *config.Config) *ShellTool {
	timeout := cfg.Tools.Shell.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &ShellTool{
		cfg:       cfg,
		workspace: cfg.ResolvePath(cfg.Tools.Shell.WorkingDir),
		timeout:   timeout,
	}
}

// NewShellToolWithSessionManager 创建带 session manager 的 Shell 工具
func NewShellToolWithSessionManager(cfg *config.Config, mgr *ExecSessionManager) *ShellTool {
	timeout := cfg.Tools.Shell.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &ShellTool{
		cfg:            cfg,
		workspace:      cfg.ResolvePath(cfg.Tools.Shell.WorkingDir),
		timeout:        timeout,
		sessionManager: mgr,
	}
}

// SetSessionManager sets the session manager
func (t *ShellTool) SetSessionManager(mgr *ExecSessionManager) {
	t.sessionManager = mgr
}

// SetSessionKey sets the owner session key
func (t *ShellTool) SetSessionKey(key string) {
	t.sessionKey = key
}

// Name 工具名称
func (t *ShellTool) Name() string {
	return "shell"
}

// Description 工具描述
func (t *ShellTool) Description() string {
	return "Execute shell commands in the workspace. Use yield_time_ms for long-running or interactive commands."
}

// Parameters 参数定义
func (t *ShellTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{
		"command":          StringParam("The shell command to execute"),
		"timeout":          NumberParam("Timeout in seconds (default: 30)"),
		"yield_time_ms":    NumberParam("Optional milliseconds to wait before returning. When set, returns a session_id that can be polled with write_stdin."),
		"max_output_chars": NumberParam("Maximum output characters to return when yield_time_ms is used (default 10000, max 50000)."),
	}, []string{"command"})
}

// Execute 执行工具
func (t *ShellTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Command         string  `json:"command"`
		Timeout         float64 `json:"timeout,omitempty"`
		YieldTimeMs     int     `json:"yield_time_ms,omitempty"`
		MaxOutputChars  int     `json:"max_output_chars,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Check if we're in session mode
	if params.YieldTimeMs > 0 && t.sessionManager != nil {
		execTimeout := t.timeout
		if params.Timeout > 0 {
			execTimeout = time.Duration(params.Timeout) * time.Second
		}

		yieldMs := clampInt(params.YieldTimeMs, DefaultYieldMs, 0, MaxYieldMs)
		maxOutput := clampInt(params.MaxOutputChars, DefaultMaxOutput, 1000, MaxMaxOutput)

		logger.Debug("Shell command (session)",
			logger.String("command", params.Command),
			logger.String("workspace", t.workspace),
		)

		sessionID, poll, err := t.sessionManager.Start(
			ctx,
			t.cfg,
			params.Command,
			t.workspace,
			execTimeout,
			yieldMs,
			maxOutput,
			t.sessionKey,
		)
		if err != nil {
			res, _ := json.Marshal(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return res, nil
		}

		result := FormatSessionPoll(sessionID, poll)
		res, _ := json.Marshal(result)
		return res, nil
	}

	// Normal one-shot execution
	execTimeout := t.timeout
	if params.Timeout > 0 {
		execTimeout = time.Duration(params.Timeout) * time.Second
	}

	// 创建带超时的 context
	execCtx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	// 应用安全检查
	command := params.Command
	if t.cfg.Security.RestrictToWorkspace {
		var err error
		command, err = security.FilterCommandForPathSafety(command, t.workspace)
		if err != nil {
			result := map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			res, _ := json.Marshal(result)
			return res, nil
		}

		allowLoopback := t.cfg.Security.WebUIAllowLocalServiceAccess && security.CurrentScopeAllowsLoopback(t.cfg.Security.WebUIAllowLocalServiceAccess)
		if security.ContainsInternalURL(command, allowLoopback) {
			result := map[string]interface{}{
				"success": false,
				"error":   "Command contains URL targeting internal/private address",
			}
			res, _ := json.Marshal(result)
			return res, nil
		}
	}

	// 应用沙盒包装（如果配置了）
	if t.cfg.Tools.Shell.Sandbox != "" && runtime.GOOS != "windows" {
		workspace := t.workspace
		mediaDir := t.cfg.Tools.Shell.MediaDir
		registry := GetSandboxRegistry(mediaDir)
		var err error
		command, err = registry.WrapCommand(t.cfg.Tools.Shell.Sandbox, command, workspace, workspace)
		if err != nil {
			logger.Debug("Sandbox wrap failed, falling back to normal execution", logger.ErrorField(err))
			// 沙盒失败时回退到正常执行
			command = params.Command
		}
	}

	logger.Debug("Shell command",
		logger.String("command", command),
		logger.String("workspace", t.workspace),
	)

	// 确定 shell
	var shellCmd string
	var shellArgs []string

	switch runtime.GOOS {
	case "windows":
		if bytes.Contains([]byte(command), []byte("\n")) {
			shellCmd = "powershell"
			shellArgs = []string{"-NoProfile", "-Command", command}
		} else {
			shellCmd = "cmd.exe"
			shellArgs = []string{"/c", command}
		}
	default:
		shellCmd = "bash"
		shellArgs = []string{"-c", command}
	}

	// 创建命令
	cmd := exec.CommandContext(execCtx, shellCmd, shellArgs...)
	cmd.Dir = t.workspace
	cmd.Env = buildEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := map[string]any{
		"success":     err == nil,
		"command":     params.Command,
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
		"duration_ms": duration.Milliseconds(),
	}

	if err != nil {
		result["error"] = err.Error()
		if execCtx.Err() == context.DeadlineExceeded {
			result["error"] = "command timed out"
		}
	}

	res, _ := json.Marshal(result)
	return res, nil
}
