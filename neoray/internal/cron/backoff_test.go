package cron

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestBackoffPeriods 退避应指数增长并封顶
func TestBackoffPeriods(t *testing.T) {
	cases := map[int]int{0: 0, 1: 1, 2: 2, 3: 4, 4: 8, 10: 8}
	for fails, want := range cases {
		if got := backoffPeriods(fails); got != want {
			t.Fatalf("失败 %d 次应退避 %d 个周期，实际 %d", fails, want, got)
		}
	}
}

// TestApplyFailureBackoffEvery every 任务失败后跳过对应周期
func TestApplyFailureBackoffEvery(t *testing.T) {
	now := nowMS()
	schedule := CronSchedule{Kind: ScheduleKindEvery, EveryMS: int64(time.Hour / time.Millisecond)}
	next := now + schedule.EveryMS

	if got := applyFailureBackoff(schedule, next, 0, now); got != next {
		t.Fatalf("无失败不应退避，实际偏移 %dms", got-next)
	}
	if got := applyFailureBackoff(schedule, next, 1, now); got-next != schedule.EveryMS {
		t.Fatalf("1 次失败应多等 1 个周期，实际 %dms", got-next)
	}
	if got := applyFailureBackoff(schedule, next, 3, now); got-next != 4*schedule.EveryMS {
		t.Fatalf("3 次失败应多等 4 个周期，实际 %dms", got-next)
	}
	// 上限 6 小时
	if got := applyFailureBackoff(schedule, next, 10, now); got-next > int64(6*time.Hour/time.Millisecond) {
		t.Fatalf("退避超过 6 小时上限: %dms", got-next)
	}
}

// TestApplyFailureBackoffCron cron 表达式按相邻触发间隔换算
func TestApplyFailureBackoffCron(t *testing.T) {
	now := nowMS()
	schedule := CronSchedule{Kind: ScheduleKindCron, Expr: "0 * * * *"} // 每小时
	next := computeNextRun(schedule, now)
	if next == nil {
		t.Fatal("cron 下次执行时间为空")
	}
	got := applyFailureBackoff(schedule, *next, 1, now)
	if got-*next != int64(time.Hour/time.Millisecond) {
		t.Fatalf("cron 任务 1 次失败应多等 1 小时，实际 %dms", got-*next)
	}
}

// TestCronJobFailureBackoffAndReset 连续失败累加、成功复位
func TestCronJobFailureBackoffAndReset(t *testing.T) {
	failing := true
	handler := func(ctx context.Context, job *CronJob) error {
		if failing {
			return errors.New("boom")
		}
		return nil
	}
	cs := newTestScheduler(t, handler, dueJob("job1"))

	// 第一次失败
	cs.onTimer()
	cs.wg.Wait()

	cs.mu.RLock()
	fails := cs.store.Jobs[0].State.ConsecutiveFails
	nextAfterFail := cs.store.Jobs[0].State.NextRunAtMS
	cs.mu.RUnlock()
	if fails != 1 {
		t.Fatalf("连续失败应为 1，实际 %d", fails)
	}

	// 成功一次应复位
	failing = false
	cs.mu.Lock()
	cs.store.Jobs[0].State.NextRunAtMS = nowMS() - 1000
	cs.mu.Unlock()
	if err := cs.saveStore(); err != nil {
		t.Fatal(err)
	}
	cs.onTimer()
	cs.wg.Wait()

	cs.mu.RLock()
	job := cs.store.Jobs[0]
	cs.mu.RUnlock()
	if job.State.ConsecutiveFails != 0 {
		t.Fatalf("成功后失败计数应清零，实际 %d", job.State.ConsecutiveFails)
	}
	if job.State.NextRunAtMS >= nextAfterFail {
		t.Fatalf("成功后的下次执行应早于失败退避时间（%d >= %d）", job.State.NextRunAtMS, nextAfterFail)
	}
}
