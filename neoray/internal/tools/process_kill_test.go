package tools

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestKillTerminatesGrandchildren 杀会话时，脚本拉起的后台子进程也要被结束。
// 回归：只 kill bash 会让 `sleep 60 &` 变成孤儿继续运行。
func TestKillTerminatesGrandchildren(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("环境无 bash")
	}

	home := t.TempDir()
	t.Setenv("NEORAY_HOME", home)
	pidFile := filepath.Join(home, "child.pid")

	cfg := testConfigWithWorkspace(t, home)
	mgr := NewExecSessionManager()
	sessID, _, err := mgr.Start(context.Background(), cfg,
		"sleep 60 & echo $! > "+pidFile+"; wait",
		home, 30*time.Second, 50, 4000, "test")
	if err != nil {
		t.Skipf("无法启动会话: %v", err)
	}

	// 等子进程写下 PID
	var childPID int
	for i := 0; i < 50; i++ {
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, perr := strconv.Atoi(strings.TrimSpace(string(data))); perr == nil && pid > 0 {
				childPID = pid
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if childPID == 0 {
		t.Skip("子进程未启动，跳过")
	}

	mgr.mu.Lock()
	session := mgr.sessions[sessID]
	mgr.mu.Unlock()
	if session == nil {
		t.Fatal("会话不存在")
	}
	session.kill()

	// 等一会儿确认子进程已消失
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPID, 0); err != nil {
			return // 已经不存在
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("子进程 %d 仍在运行（只杀了 bash，没杀进程组）", childPID)
}
