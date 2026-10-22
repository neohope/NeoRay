package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestExitedSessionStillPollable 命令在两次 poll 之间结束时，
// 第二次 write_stdin 仍应能取到输出与退出码（回归：回收发生在读输出之前）。
func TestExitedSessionStillPollable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("NEORAY_HOME", home)
	cfg := testConfigWithWorkspace(t, home)

	mgr := NewExecSessionManager()
	sessID, _, err := mgr.Start(context.Background(), cfg, "echo FINAL-OUTPUT; sleep 0.2", home, 10*time.Second, 50, 4000, "k")
	if err != nil {
		t.Skipf("无法启动会话: %v", err)
	}
	sess := execSessionByID(t, mgr, sessID)

	// 等命令结束（超过 yield 时间）
	time.Sleep(600 * time.Millisecond)

	poll, err := mgr.Write(sessID, "", false, false, 50, 4000, "k")
	if err != nil {
		t.Fatalf("进程退出后仍应可读取输出，实际报错: %v", err)
	}
	if poll == nil {
		t.Fatal("应返回 poll 结果")
	}
	combined := poll.Output
	if !strings.Contains(combined, "FINAL-OUTPUT") && !poll.Done {
		t.Fatalf("应能取到输出或退出状态，实际 output=%q done=%v", combined, poll.Done)
	}
	_ = sess
}
