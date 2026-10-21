package cron

import (
	"testing"
	"time"
)

// TestBuildScheduleFromIntervalDuration 时长写法（默认配置 "1h"）必须转成 every
func TestBuildScheduleFromIntervalDuration(t *testing.T) {
	for _, tc := range []struct {
		spec string
		want time.Duration
	}{
		{"1h", time.Hour},
		{"30m", 30 * time.Minute},
		{"90s", 90 * time.Second},
	} {
		s, err := BuildScheduleFromInterval(tc.spec)
		if err != nil {
			t.Fatalf("%s 解析失败: %v", tc.spec, err)
		}
		if s.Kind != ScheduleKindEvery {
			t.Fatalf("%s 应转成 every，实际 %v（这正是梦中任务注册失败的根因）", tc.spec, s.Kind)
		}
		if s.EveryMS != tc.want.Milliseconds() {
			t.Fatalf("%s 间隔应为 %v，实际 %dms", tc.spec, tc.want, s.EveryMS)
		}
	}
}

// TestBuildScheduleFromIntervalCron cron 表达式仍然支持
func TestBuildScheduleFromIntervalCron(t *testing.T) {
	s, err := BuildScheduleFromInterval("0 * * * *")
	if err != nil {
		t.Fatalf("cron 表达式解析失败: %v", err)
	}
	if s.Kind != ScheduleKindCron || s.Expr != "0 * * * *" {
		t.Fatalf("cron 解析错误: %+v", s)
	}
}

// TestBuildScheduleFromIntervalInvalid 非法值应报错而不是静默注册失败
func TestBuildScheduleFromIntervalInvalid(t *testing.T) {
	for _, spec := range []string{"", "1x", "every hour", "0 0"} {
		if _, err := BuildScheduleFromInterval(spec); err == nil {
			t.Fatalf("%q 应报错", spec)
		}
	}
}
