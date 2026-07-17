package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/security"
)

const (
	defaultShellTimeout = 30 * time.Second
)

// ShellTool Shell 执行工具
type ShellTool struct {
	cfg       *config.Config
	workspace string
	timeout   time.Duration

	mu             sync.RWMutex
	sessionManager *ExecSessionManager
	sessionKey     string
}

// NewShellTool 创建 Shell 工具
func NewShellTool(cfg *config.Config) *ShellTool {
	timeout := cfg.Tools.Shell.Timeout
	if timeout == 0 {
		timeout = defaultShellTimeout
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
		timeout = defaultShellTimeout
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
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionManager = mgr
}

// SetSessionKey sets the owner session key
func (t *ShellTool) SetSessionKey(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
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

	// Check blocked_commands
	if err := checkBlockedCommands(params.Command, t.cfg.Tools.Shell.BlockedCommands); err != nil {
		result := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		res, _ := json.Marshal(result)
		return res, nil
	}

	// Check if we're in session mode
	t.mu.RLock()
	sessMgr := t.sessionManager
	sessKey := t.sessionKey
	t.mu.RUnlock()

	if params.YieldTimeMs > 0 && sessMgr != nil {
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

		sessionID, poll, err := sessMgr.Start(
			ctx,
			t.cfg,
			params.Command,
			t.workspace,
			execTimeout,
			yieldMs,
			maxOutput,
			sessKey,
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
	var finalCommand string = command
	if t.cfg.Security.RestrictToWorkspace {
		var err error
		// Filter the original command first
		filteredCmd, err := security.FilterCommandForPathSafety(command, t.workspace)
		if err != nil {
			result := map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			}
			res, _ := json.Marshal(result)
			return res, nil
		}

		allowLoopback := t.cfg.Security.WebUIAllowLocalServiceAccess && security.CurrentScopeAllowsLoopback(ctx, t.cfg.Security.WebUIAllowLocalServiceAccess)
		if security.ContainsInternalURL(filteredCmd, allowLoopback) {
			result := map[string]interface{}{
				"success": false,
				"error":   "Command contains URL targeting internal/private address",
			}
			res, _ := json.Marshal(result)
			return res, nil
		}
		finalCommand = filteredCmd
	}

	// 应用沙盒包装（如果配置了） - ALWAYS use the filtered command
	if t.cfg.Tools.Shell.Sandbox != "" && runtime.GOOS != "windows" {
		workspace := t.workspace
		mediaDir := t.cfg.Tools.Shell.MediaDir
		registry := GetSandboxRegistry(mediaDir)
		// Wrap the already filtered command
		wrappedCmd, err := registry.WrapCommand(t.cfg.Tools.Shell.Sandbox, finalCommand, workspace, workspace)
		if err != nil {
			// P1-6: 沙盒配置为必需时，失败应拒绝执行而非回退到非沙盒模式。
			// 回退会静默降低安全保护级别。
			result := map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Sandbox '%s' is configured but failed to initialize: %v. Refusing to execute without sandbox.", t.cfg.Tools.Shell.Sandbox, err),
			}
			res, _ := json.Marshal(result)
			return res, nil
		}
		finalCommand = wrappedCmd
	}

	logger.Debug("Shell command",
		logger.String("command", finalCommand),
		logger.String("workspace", t.workspace),
	)

	// 确定 shell
	var shellCmd string
	var shellArgs []string

	switch runtime.GOOS {
	case "windows":
		// 统一使用 PowerShell -EncodedCommand 避免命令注入：
		// cmd.exe /c 会解释 shell 元字符（|, &, >, ^, % 等），存在注入风险
		shellCmd = "powershell"
		encodedCmd := encodePowerShellCommand(finalCommand)
		shellArgs = []string{"-NoProfile", "-EncodedCommand", encodedCmd}
	default:
		// P0-fix: 使用 bash -s 从 stdin 读取命令，避免 bash -c 的命令注入风险。
		// 命令通过 Stdin 管道传递，不经过 shell 解释，确保原始命令不被二次解析。
		shellCmd = "bash"
		shellArgs = []string{"-s"}
	}

	// 创建命令
	cmd := exec.CommandContext(execCtx, shellCmd, shellArgs...)
	cmd.Dir = t.workspace
	cmd.Env = buildEnv()

	// Unix: 通过 stdin 传递命令（bash -s 从 stdin 读取）
	if runtime.GOOS != "windows" {
		cmd.Stdin = strings.NewReader(finalCommand)
	}

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

// encodePowerShellCommand 将命令编码为 PowerShell -EncodedCommand 所需的 Base64 UTF-16LE 格式
// 这避免了命令注入问题，因为特殊字符（$()、反引号、分号等）不会被解释
func encodePowerShellCommand(cmd string) string {
	// PowerShell -EncodedCommand 期望 UTF-16LE 编码的 Base64
	encoded := utf16.Encode([]rune(cmd))
	buf := make([]byte, 2*len(encoded))
	for i, r := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// checkBlockedCommands 检查命令是否匹配 blocked_commands 列表
func checkBlockedCommands(command string, blockedCommands []string) error {
	if len(blockedCommands) == 0 {
		return nil
	}
	cmdLower := strings.ToLower(strings.TrimSpace(command))
	for _, blocked := range blockedCommands {
		if blocked == "" {
			continue
		}
		blockedLower := strings.ToLower(blocked)
		if strings.Contains(cmdLower, blockedLower) {
			return fmt.Errorf("command blocked: matches blocked pattern %q", blocked)
		}
	}
	return nil
}
