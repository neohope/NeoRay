package cron

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestLongJobDoesNotSpin 长任务执行期间调度循环不应空转。
// 回归：派发时不推进 NextRunAtMS → computeDelay 算成 0 → runLoop 死循环
// （实测 400ms 采样里烧掉 433ms CPU）。
func TestLongJobDoesNotSpin(t *testing.T) {
	dir := t.TempDir()
	var runs int64
	var ticks int64

	s := NewCronScheduler(filepath.Join(dir, "jobs.json"), func(ctx context.Context, job *CronJob) error {
		atomic.AddInt64(&runs, 1)
		time.Sleep(400 * time.Millisecond)
		return nil
	})
	if err := s.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer s.Stop()

	if _, err := s.AddJob("slow", CronSchedule{Kind: ScheduleKindEvery, EveryMS: 50}, "x", false, "", "", false, nil, ""); err != nil {
		t.Fatalf("添加任务失败: %v", err)
	}

	// 采样期间统计 onTimer 被调用的次数（自旋时会暴涨）
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		ticks++
		time.Sleep(10 * time.Millisecond)
	}

	// 自旋时循环体每秒会跑数十万次；这里用"任务执行次数"间接判断：
	// 任务每次 400ms，若自旋则 NextRunAtMS 不推进，任务会被反复派发。
	if got := atomic.LoadInt64(&runs); got > 3 {
		t.Fatalf("长任务被反复派发（调度器自旋）：700ms 内执行 %d 次", got)
	}
}
