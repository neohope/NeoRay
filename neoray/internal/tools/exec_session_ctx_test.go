package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"neoray/internal/config"
)

// TestExecSessionSurvivesToolCtxCancel 工具 ctx 被取消后，长任务进程不应被杀。
// 回归：此前 spawnCommand 用 exec.CommandContext(工具ctx)，Execute 一返回就 SIGKILL，
// 带 yield_time_ms 的命令在首次 poll 后必然死亡。
func TestExecSessionSurvivesToolCtxCancel(t *testing.T) {
	home := t.TempDir()
	os.Setenv("NEORAY_HOME", home)
	defer os.Unsetenv("NEORAY_HOME")

	cfg := &config.Config{}
	cfg.HomeDir = home
	cfg.Memory.Workspace = filepath.Join(home, "workspace")
	if err := os.MkdirAll(cfg.Memory.Workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	mgr := NewExecSessionManager()
	ctx, cancel := context.WithCancel(context.Background())

	sessID, _, err := mgr.Start(ctx, cfg, "sleep 3; echo done", cfg.Memory.Workspace, 10*time.Second, 50, 4000, "test")
	if err != nil {
		t.Skipf("无法启动会话（环境限制）: %v", err)
	}
	// 模拟 tools.Execute 返回：工具 ctx 被取消
	cancel()

	time.Sleep(300 * time.Millisecond)
	sess := execSessionByID(t, mgr, sessID)
	poll := sess.poll(50, 4000, true, false)
	if poll.Done {
		t.Fatalf("工具 ctx 取消后长任务不应立即结束，实际已结束（退出码 %v）", poll.ExitCode)
	}
	sess.kill()
}

func execSessionByID(t *testing.T, mgr *ExecSessionManager, id string) *ExecSession {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	s, ok := mgr.sessions[id]
	if !ok {
		t.Fatalf("会话 %s 不存在", id)
	}
	return s
}

// TestExecSessionTimeoutStillKills 会话自身超时仍应生效
func TestExecSessionTimeoutStillKills(t *testing.T) {
	home := t.TempDir()
	os.Setenv("NEORAY_HOME", home)
	defer os.Unsetenv("NEORAY_HOME")

	cfg := &config.Config{}
	cfg.HomeDir = home
	cfg.Memory.Workspace = filepath.Join(home, "workspace")
	_ = os.MkdirAll(cfg.Memory.Workspace, 0o755)

	mgr := NewExecSessionManager()
	sessID, _, err := mgr.Start(context.Background(), cfg, "sleep 30", cfg.Memory.Workspace, 500*time.Millisecond, 50, 4000, "test")
	if err != nil {
		t.Skipf("无法启动会话（环境限制）: %v", err)
	}
	defer func() {
		if s := execSessionByID(t, mgr, sessID); s != nil {
			s.kill()
		}
	}()

	time.Sleep(900 * time.Millisecond)
	sess := execSessionByID(t, mgr, sessID)
	poll := sess.poll(50, 4000, false, false)
	if !poll.Done && !poll.TimedOut {
		t.Fatalf("超时后应结束或标记超时，实际 done=false timedOut=false elapsed=%.2f", poll.Elapsed)
	}
}
