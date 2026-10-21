package cron

import (
	"fmt"
	"strings"
	"time"
)

// BuildScheduleFromInterval 把「间隔字符串」转成调度定义。
//
// memory.dream_interval 的默认值是 "1h"（时长写法），但历史实现把它
// 直接塞进 Expr 当 cron 表达式用，于是每次启动都报
// "Invalid cron expression: 1h"，dream 任务**从未真正注册成功**。
//
// 规则：
//   - 能按 duration 解析（"1h" / "30m" / "90s"）→ every
//   - 5/6 字段 → 按 cron 表达式
//   - 其它 → 报错，由调用方决定是告警还是回退
func BuildScheduleFromInterval(interval string) (CronSchedule, error) {
	spec := strings.TrimSpace(interval)
	if spec == "" {
		return CronSchedule{}, fmt.Errorf("empty interval")
	}

	if d, err := time.ParseDuration(spec); err == nil {
		if d <= 0 {
			return CronSchedule{}, fmt.Errorf("interval must be positive: %s", spec)
		}
		return CronSchedule{
			Kind:    ScheduleKindEvery,
			EveryMS: d.Milliseconds(),
		}, nil
	}

	if fields := strings.Fields(spec); len(fields) == 5 || len(fields) == 6 {
		return CronSchedule{
			Kind: ScheduleKindCron,
			Expr: spec,
		}, nil
	}

	return CronSchedule{}, fmt.Errorf("unsupported interval %q: 需要时长（如 1h）或 cron 表达式（如 0 * * * *）", spec)
}
