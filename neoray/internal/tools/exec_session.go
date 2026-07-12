package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/security"
)

const (
	DefaultYieldMs      = 1000
	MaxYieldMs          = 30000
	DefaultWaitForMs    = 10000
	MaxWaitForMs        = 120000
	DefaultMaxOutput    = 10000
	MaxMaxOutput        = 50000
	MaxSessions         = 8
	IdleTimeoutSeconds  = 1800
)

// SessionPoll represents a poll result from an exec session
type SessionPoll struct {
	Output         string
	Done           bool
	ExitCode       *int
	Elapsed        float64
	TimedOut       bool
	Terminated     bool
	StdinClosed    bool
	TruncatedChars int
}

// ExecSessionInfo represents information about an active exec session
type ExecSessionInfo struct {
	SessionID       string
	Command         string
	Cwd             string
	Elapsed         float64
	Idle            float64
	Remaining       float64
	ReturnCode      *int
	OwnerSessionKey string
}

// ExecSession represents a single long-running exec session
type ExecSession struct {
	sessionID       string
	cmd             *exec.Cmd
	command         string
	cwd             string
	ownerSessionKey string
	startedAt       time.Time
	deadline        time.Time
	lastAccess      time.Time
	stdin           io.WriteCloser
	stdoutPipe      io.ReadCloser
	stderrPipe      io.ReadCloser
	outputCh        chan string
	doneCh          chan struct{}
	mu              sync.Mutex
	chunks          []string
	timedOut        bool
	returnCode      *int
	wg              sync.WaitGroup
}

// newExecSession creates a new ExecSession
func newExecSession(
	sessionID string,
	cmd *exec.Cmd,
	command string,
	cwd string,
	timeout time.Duration,
	ownerSessionKey string,
) (*ExecSession, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	s := &ExecSession{
		sessionID:       sessionID,
		cmd:             cmd,
		command:         command,
		cwd:             cwd,
		ownerSessionKey: ownerSessionKey,
		startedAt:       time.Now(),
		stdin:           stdin,
		stdoutPipe:      stdout,
		stderrPipe:      stderr,
		outputCh:        make(chan string, 100),
		doneCh:          make(chan struct{}),
		chunks:          make([]string, 0),
	}

	if timeout > 0 {
		s.deadline = time.Now().Add(timeout)
	} else {
		s.deadline = time.Now().Add(8760 * time.Hour) // Far future
	}
	s.lastAccess = time.Now()

	return s, nil
}

// start starts the session
func (s *ExecSession) start() error {
	if err := s.cmd.Start(); err != nil {
		s.stdin.Close()
		s.stdoutPipe.Close()
		s.stderrPipe.Close()
		return err
	}

	s.wg.Add(2)
	go s.readStream(s.stdoutPipe, "")
	go s.readStream(s.stderrPipe, "STDERR:\n")

	go func() {
		err := s.cmd.Wait()
		close(s.doneCh)
		s.mu.Lock()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				s.returnCode = &code
			} else {
				code := -1
				s.returnCode = &code
			}
		} else {
			code := 0
			s.returnCode = &code
		}
		s.mu.Unlock()
	}()

	return nil
}

// readStream reads from a stream and sends to output channel
func (s *ExecSession) readStream(stream io.ReadCloser, prefix string) {
	defer s.wg.Done()

	scanner := bufio.NewScanner(stream)
	// Use a large buffer to handle long lines
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, cap(buf))

	first := true
	for scanner.Scan() {
		line := scanner.Text()
		var output string
		if first && prefix != "" {
			output = prefix + line + "\n"
			first = false
		} else {
			output = line + "\n"
		}
		s.mu.Lock()
		s.chunks = append(s.chunks, output)
		s.mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		logger.Debug("Error reading stream", logger.ErrorField(err))
	}
}

// write writes to the session's stdin
func (s *ExecSession) write(chars string) error {
	s.mu.Lock()
	if s.returnCode != nil {
		s.mu.Unlock()
		return fmt.Errorf("session has already exited")
	}
	s.mu.Unlock()

	_, err := s.stdin.Write([]byte(chars))
	if err != nil {
		return fmt.Errorf("write to stdin: %w", err)
	}
	return nil
}

// closeStdin closes the session's stdin
func (s *ExecSession) closeStdin() error {
	s.mu.Lock()
	if s.returnCode != nil {
		s.mu.Unlock()
		return fmt.Errorf("session has already exited")
	}
	s.mu.Unlock()

	return s.stdin.Close()
}

// kill terminates the session
func (s *ExecSession) kill() {
	s.mu.Lock()
	if s.returnCode != nil {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}

	// Wait a bit for cleanup
	select {
	case <-s.doneCh:
	case <-time.After(5 * time.Second):
	}
}

// poll polls the session for output
func (s *ExecSession) poll(
	yieldTimeMs int,
	maxOutputChars int,
	terminated bool,
	stdinClosed bool,
) *SessionPoll {
	s.mu.Lock()
	s.lastAccess = time.Now()
	s.mu.Unlock()

	if yieldTimeMs > 0 {
		waitMs := yieldTimeMs
		if waitMs > MaxYieldMs {
			waitMs = MaxYieldMs
		}
		select {
		case <-time.After(time.Duration(waitMs) * time.Millisecond):
		case <-s.doneCh:
		}
	}

	// Check for timeout
	s.mu.Lock()
	if time.Now().After(s.deadline) && s.returnCode == nil {
		s.timedOut = true
		s.mu.Unlock()
		s.kill()
	} else {
		s.mu.Unlock()
	}

	// Wait for read goroutines if done
	select {
	case <-s.doneCh:
		s.wg.Wait()
	default:
	}

	s.mu.Lock()
	chunks := make([]string, len(s.chunks))
	copy(chunks, s.chunks)
	s.chunks = s.chunks[:0]
	exitCode := s.returnCode
	timedOut := s.timedOut
	s.mu.Unlock()

	output := stringsJoin(chunks, "")
	output, truncated := truncateOutput(output, maxOutputChars)

	elapsed := time.Since(s.startedAt).Seconds()

	return &SessionPoll{
		Output:         output,
		Done:           exitCode != nil,
		ExitCode:       exitCode,
		Elapsed:        elapsed,
		TimedOut:       timedOut,
		Terminated:     terminated,
		StdinClosed:    stdinClosed,
		TruncatedChars: truncated,
	}
}

// ExecSessionManager manages multiple exec sessions
type ExecSessionManager struct {
	maxSessions int
	idleTimeout time.Duration
	mu          sync.Mutex
	sessions    map[string]*ExecSession
}

// NewExecSessionManager creates a new ExecSessionManager
func NewExecSessionManager() *ExecSessionManager {
	return &ExecSessionManager{
		maxSessions: MaxSessions,
		idleTimeout: IdleTimeoutSeconds * time.Second,
		sessions:    make(map[string]*ExecSession),
	}
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Start starts a new exec session
func (m *ExecSessionManager) Start(
	ctx context.Context,
	cfg *config.Config,
	command string,
	cwd string,
	timeout time.Duration,
	yieldTimeMs int,
	maxOutputChars int,
	ownerSessionKey string,
) (string, *SessionPoll, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up stale sessions
	m.cleanupLocked()

	if len(m.sessions) >= m.maxSessions {
		return "", nil, fmt.Errorf("maximum exec sessions reached (%d)", m.maxSessions)
	}

	// Spawn the process
	cmd, err := spawnCommand(ctx, command, cwd, cfg, true)
	if err != nil {
		return "", nil, err
	}

	sessionID := generateSessionID()
	session, err := newExecSession(
		sessionID,
		cmd,
		command,
		cwd,
		timeout,
		ownerSessionKey,
	)
	if err != nil {
		return "", nil, err
	}

	if err := session.start(); err != nil {
		return "", nil, err
	}

	m.sessions[sessionID] = session

	poll := session.poll(yieldTimeMs, maxOutputChars, false, false)

	// If done immediately, remove from sessions
	if poll.Done {
		delete(m.sessions, sessionID)
	}

	return sessionID, poll, nil
}

// Write writes to an exec session
func (m *ExecSessionManager) Write(
	sessionID string,
	chars string,
	closeStdin bool,
	terminate bool,
	yieldTimeMs int,
	maxOutputChars int,
	ownerSessionKey string,
) (*SessionPoll, error) {
	m.mu.Lock()
	session := m.sessions[sessionID]
	if session == nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Check owner
	if ownerSessionKey != "" && session.ownerSessionKey != "" && session.ownerSessionKey != ownerSessionKey {
		m.mu.Unlock()
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Clean up stale sessions
	m.cleanupLocked()

	// Re-validate session still exists after cleanup
	if m.sessions[sessionID] == nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	m.mu.Unlock()

	var writeErr error
	if chars != "" {
		writeErr = session.write(chars)
		if writeErr != nil {
			return nil, writeErr
		}
	}

	var stdinClosed bool
	if closeStdin {
		if err := session.closeStdin(); err == nil {
			stdinClosed = true
		}
	}

	if terminate {
		session.kill()
	}

	poll := session.poll(yieldTimeMs, maxOutputChars, terminate, stdinClosed)

	if poll.Done {
		m.mu.Lock()
		// Only delete if the session in the map is still the same object
		if m.sessions[sessionID] == session {
			delete(m.sessions, sessionID)
		}
		m.mu.Unlock()
	}

	return poll, nil
}

// List lists active sessions
func (m *ExecSessionManager) List(ownerSessionKey string) []ExecSessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupLocked()

	var infos []ExecSessionInfo
	now := time.Now()

	for id, session := range m.sessions {
		session.mu.Lock()
		info := ExecSessionInfo{
			SessionID:       id,
			Command:         session.command,
			Cwd:             session.cwd,
			Elapsed:         now.Sub(session.startedAt).Seconds(),
			Idle:            now.Sub(session.lastAccess).Seconds(),
			Remaining:       session.deadline.Sub(now).Seconds(),
			ReturnCode:      session.returnCode,
			OwnerSessionKey: session.ownerSessionKey,
		}
		session.mu.Unlock()

		// Filter by owner
		if ownerSessionKey == "" || session.ownerSessionKey == "" || session.ownerSessionKey == ownerSessionKey {
			infos = append(infos, info)
		}
	}

	return infos
}

// cleanupLocked cleans up stale sessions (must hold mu)
func (m *ExecSessionManager) cleanupLocked() {
	now := time.Now()
	var stale []string
	for id, session := range m.sessions {
		session.mu.Lock()
		if now.Sub(session.lastAccess) > m.idleTimeout {
			stale = append(stale, id)
		}
		session.mu.Unlock()
	}

	for _, id := range stale {
		if session := m.sessions[id]; session != nil {
			session.kill()
		}
		delete(m.sessions, id)
	}
}

// spawnCommand spawns a command (similar to ShellTool)
func spawnCommand(ctx context.Context, command string, cwd string, cfg *config.Config, withStdin bool) (*exec.Cmd, error) {
	workspace := cfg.ResolvePath(cfg.Tools.Shell.WorkingDir)
	if workspace == "" {
		workspace = cwd
	}

	// 应用安全检查
	if cfg.Security.RestrictToWorkspace {
		var err error
		command, err = security.FilterCommandForPathSafety(command, workspace)
		if err != nil {
			return nil, err
		}

		allowLoopback := cfg.Security.WebUIAllowLocalServiceAccess && security.CurrentScopeAllowsLoopback(cfg.Security.WebUIAllowLocalServiceAccess)
		if security.ContainsInternalURL(command, allowLoopback) {
			return nil, fmt.Errorf("command contains URL targeting internal/private address")
		}
	}

	// 应用沙盒包装（如果配置了）
	if cfg.Tools.Shell.Sandbox != "" && runtime.GOOS != "windows" {
		mediaDir := cfg.Tools.Shell.MediaDir
		registry := GetSandboxRegistry(mediaDir)
		var err error
		command, err = registry.WrapCommand(cfg.Tools.Shell.Sandbox, command, workspace, cwd)
		if err != nil {
			logger.Debug("Sandbox wrap failed, falling back to normal execution", logger.ErrorField(err))
			// 沙盒失败时回退到正常执行
		}
	}

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

	cmd := exec.CommandContext(ctx, shellCmd, shellArgs...)
	cmd.Dir = cwd

	// Environment
	cmd.Env = buildEnv()

	return cmd, nil
}

// buildEnv builds environment variables for subprocess
func buildEnv() []string {
	if runtime.GOOS == "windows" {
		sr := os.Getenv("SYSTEMROOT")
		if sr == "" {
			sr = "C:\\Windows"
		}
		env := []string{
			"SYSTEMROOT=" + sr,
			"COMSPEC=" + os.Getenv("COMSPEC"),
			"USERPROFILE=" + os.Getenv("USERPROFILE"),
			"HOMEDRIVE=" + os.Getenv("HOMEDRIVE"),
			"HOMEPATH=" + os.Getenv("HOMEPATH"),
			"TEMP=" + os.Getenv("TEMP"),
			"TMP=" + os.Getenv("TMP"),
			"PATHEXT=" + os.Getenv("PATHEXT"),
			"PATH=" + os.Getenv("PATH"),
			"PYTHONUNBUFFERED=1",
		}
		return env
	}

	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "C.UTF-8"
	}
	term := os.Getenv("TERM")
	if term == "" {
		term = "dumb"
	}
	return []string{
		"HOME=" + home,
		"LANG=" + lang,
		"TERM=" + term,
		"PYTHONUNBUFFERED=1",
	}
}

// truncateOutput truncates output to max chars
func truncateOutput(output string, maxChars int) (string, int) {
	if len(output) <= maxChars {
		return output, 0
	}
	half := maxChars / 2
	omitted := len(output) - maxChars
	return output[:half] + fmt.Sprintf("\n\n... (%d chars truncated) ...\n\n", omitted) + output[len(output)-half:], omitted
}

// stringsJoin joins strings (helper)
func stringsJoin(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	for i, s := range strs {
		if i > 0 {
			buf.WriteString(sep)
		}
		buf.WriteString(s)
	}
	return buf.String()
}

// clampInt clamps an integer value
func clampInt(value int, defaultValue int, minimum int, maximum int) int {
	if value == 0 {
		return defaultValue
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

// FormatSessionPoll formats a session poll result
func FormatSessionPoll(sessionID string, poll *SessionPoll) string {
	var parts []string
	if poll.Output != "" {
		parts = append(parts, poll.Output)
	}
	if poll.TruncatedChars > 0 {
		parts = append(parts, fmt.Sprintf("(output truncated by %d chars)", poll.TruncatedChars))
	}
	if poll.TimedOut {
		parts = append(parts, "Error: Command timed out; session was terminated.")
	}
	if poll.Terminated && !poll.TimedOut {
		parts = append(parts, "Session terminated.")
	}
	if poll.StdinClosed {
		parts = append(parts, "Stdin closed.")
	}
	if poll.Done {
		if poll.ExitCode != nil {
			parts = append(parts, fmt.Sprintf("Exit code: %d", *poll.ExitCode))
		} else {
			parts = append(parts, "Exit code: unknown")
		}
	} else {
		parts = append(parts, fmt.Sprintf("Process running. session_id: %s", sessionID))
	}
	parts = append(parts, fmt.Sprintf("Elapsed: %.1fs", poll.Elapsed))
	return stringsJoin(parts, "\n")
}
