package cron

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestScheduler 构造一个只含内存 store 的调度器（不启动 runLoop）
func newTestScheduler(t *testing.T, handler JobHandler, jobs ...CronJob) *CronScheduler {
	t.Helper()
	dir := t.TempDir()
	cs := NewCronScheduler(filepath.Join(dir, "jobs.json"), handler)
	cs.store = &CronStore{Version: storeVersion, Jobs: jobs}
	// onTimer 会先 loadStore（覆盖内存 store），因此先把测试任务落盘
	if err := cs.saveStore(); err != nil {
		t.Fatalf("准备 cron store 失败: %v", err)
	}
	return cs
}

func dueJob(id string) CronJob {
	return CronJob{
		ID:      id,
		Name:    id,
		Enabled: true,
		Schedule: CronSchedule{
			Kind:    ScheduleKindEvery,
			EveryMS: int64(time.Hour / time.Millisecond),
		},
		State: CronJobState{NextRunAtMS: nowMS() - 1000},
	}
}

// TestOnTimerRunsJobsConcurrently 一个慢任务不应阻塞另一个到期任务
func TestOnTimerRunsJobsConcurrently(t *testing.T) {
	var slowStarted, fastDone int32
	handler := func(ctx context.Context, job *CronJob) error {
		switch job.ID {
		case "slow":
			atomic.StoreInt32(&slowStarted, 1)
			time.Sleep(300 * time.Millisecond)
		case "fast":
			atomic.StoreInt32(&fastDone, 1)
		}
		return nil
	}

	cs := newTestScheduler(t, handler, dueJob("slow"), dueJob("fast"))

	start := time.Now()
	cs.onTimer()

	// 立刻检查：fast 应该在 slow 还没跑完时就已完成
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) && atomic.LoadInt32(&fastDone) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&fastDone) == 0 {
		t.Fatal("fast 任务被 slow 阻塞了（并发化未生效）")
	}
	if atomic.LoadInt32(&slowStarted) == 0 {
		t.Fatal("slow 任务未启动")
	}

	cs.wg.Wait()
	if elapsed := time.Since(start); elapsed > 900*time.Millisecond {
		t.Fatalf("总耗时 %v，说明任务仍是串行", elapsed)
	}
}

// TestOnTimerSkipsReentrant 同一任务上一次未结束时不再叠加
func TestOnTimerSkipsReentrant(t *testing.T) {
	var runs int32
	handler := func(ctx context.Context, job *CronJob) error {
		atomic.AddInt32(&runs, 1)
		time.Sleep(200 * time.Millisecond)
		return nil
	}
	cs := newTestScheduler(t, handler, dueJob("job1"))

	cs.onTimer() // 第一次：启动
	cs.onTimer() // 第二次：应被 runningJobs 跳过
	cs.wg.Wait()

	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("应只执行 1 次，实际 %d 次（重入保护失效）", got)
	}
}

// TestStopTimeoutWithLongJob Stop 不应被长任务无限拖住
func TestStopTimeoutWithLongJob(t *testing.T) {
	// 缩短超时，避免单测等 30 秒
	oldTimeout := stopTimeout
	stopTimeout = 300 * time.Millisecond
	defer func() { stopTimeout = oldTimeout }()

	blocker := make(chan struct{})
	handler := func(ctx context.Context, job *CronJob) error {
		<-blocker
		return nil
	}
	cs := newTestScheduler(t, handler, dueJob("long"))
	cs.running = true
	cs.onTimer() // 启动长任务（会一直阻塞）

	done := make(chan struct{})
	go func() {
		cs.Stop()
		close(done)
	}()

	start := time.Now()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		close(blocker)
		t.Fatal("Stop 被长任务无限阻塞（超时保护失效）")
	}
	elapsed := time.Since(start)
	close(blocker)
	cs.wg.Wait()

	if elapsed > 2*time.Second {
		t.Fatalf("Stop 应在 %v 左右返回，实际 %v", stopTimeout, elapsed)
	}
	if elapsed < 200*time.Millisecond {
		t.Fatalf("Stop 过早返回（%v），超时保护可能没生效就直接放弃了", elapsed)
	}
}

// TestJobStatePersistedByID 执行后按 ID 回写状态并推进 NextRun
func TestJobStatePersistedByID(t *testing.T) {
	cs := newTestScheduler(t, func(ctx context.Context, job *CronJob) error { return nil }, dueJob("j1"))
	cs.onTimer()
	cs.wg.Wait()

	cs.mu.RLock()
	defer cs.mu.RUnlock()
	job := cs.store.Jobs[0]
	if job.State.LastStatus != RunStatusOK {
		t.Fatalf("状态应为 ok，实际 %q", job.State.LastStatus)
	}
	if job.State.NextRunAtMS <= nowMS() {
		t.Fatal("NextRunAtMS 未推进到未来")
	}
}

// TestConcurrentStateUpdates 多任务并发执行时状态更新不应竞态（配合 -race）
func TestConcurrentStateUpdates(t *testing.T) {
	var wg sync.WaitGroup
	handler := func(ctx context.Context, job *CronJob) error {
		wg.Done()
		return nil
	}
	jobs := make([]CronJob, 0, 5)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		jobs = append(jobs, dueJob(id))
		wg.Add(1)
	}
	cs := newTestScheduler(t, handler, jobs...)
	cs.onTimer()
	wg.Wait()
	cs.wg.Wait()

	cs.mu.RLock()
	defer cs.mu.RUnlock()
	for _, job := range cs.store.Jobs {
		if job.State.LastStatus != RunStatusOK {
			t.Fatalf("%s 状态异常: %q", job.ID, job.State.LastStatus)
		}
	}
}
