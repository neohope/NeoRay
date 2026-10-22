package cron

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestRuntimeAddJobKeepsSchedulerAlive 运行期新增任务后，调度器必须继续工作。
// 回归：此前 AddJob 只 timer.Stop() 不唤醒，runLoop 永久阻塞，
// dream / autocompact / 用户任务全部静默停摆。
func TestRuntimeAddJobKeepsSchedulerAlive(t *testing.T) {
	dir := t.TempDir()
	var runs int64
	s := NewCronScheduler(filepath.Join(dir, "jobs.json"), func(ctx context.Context, job *CronJob) error {
		atomic.AddInt64(&runs, 1)
		return nil
	})
	if err := s.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer s.Stop()

	sched := CronSchedule{Kind: ScheduleKindEvery, EveryMS: 200}
	if _, err := s.AddJob("probe", sched, "hi", false, "", "", false, nil, ""); err != nil {
		t.Fatalf("添加任务失败: %v", err)
	}
	time.Sleep(700 * time.Millisecond)
	before := atomic.LoadInt64(&runs)
	if before == 0 {
		t.Fatalf("首个任务没有执行（调度器未工作）")
	}

	// 运行期再新增一个任务：历史实现会让调度器死在这里
	if _, err := s.AddJob("probe2", sched, "hi2", false, "", "", false, nil, ""); err != nil {
		t.Fatalf("第二次添加失败: %v", err)
	}

	time.Sleep(800 * time.Millisecond)
	after := atomic.LoadInt64(&runs)
	if after == before {
		t.Fatalf("运行期新增任务后调度器停摆：新增前 %d 次，之后 800ms 内 0 次", before)
	}
}

// TestWakeIsNonBlocking 唤醒信号重复投递不应阻塞
func TestWakeIsNonBlocking(t *testing.T) {
	s := NewCronScheduler(filepath.Join(t.TempDir(), "jobs.json"), nil)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			s.wake()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("wake 阻塞了（应当非阻塞丢信号）")
	}
}
