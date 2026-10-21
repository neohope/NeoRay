package agent

import (
	"testing"
	"time"

	"neoray/internal/config"
)

// TestEffectiveToolTimeoutAuto 外层超时要跟着内层工具超时走，不能固定 30s
func TestEffectiveToolTimeoutAuto(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Shell.Timeout = 60 * time.Second

	got := effectiveToolTimeout(cfg)
	if got <= cfg.Tools.Shell.Timeout {
		t.Fatalf("外层超时 %v 必须大于 shell 超时 %v，否则命令会被提前掐断", got, cfg.Tools.Shell.Timeout)
	}

	// 内层更小的情况：保持默认 30s
	cfg2 := &config.Config{}
	cfg2.Tools.Shell.Timeout = 5 * time.Second
	if got := effectiveToolTimeout(cfg2); got != defaultToolExecutionTimeout {
		t.Fatalf("内层更小时应保留默认值，实际 %v", got)
	}
}

// TestEffectiveToolTimeoutExplicit agent.tool_timeout 显式配置优先
func TestEffectiveToolTimeoutExplicit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.ToolTimeout = 90 * time.Second
	cfg.Tools.Shell.Timeout = 200 * time.Second

	if got := effectiveToolTimeout(cfg); got != 90*time.Second {
		t.Fatalf("显式配置应优先，实际 %v", got)
	}
}

// TestEffectiveToolTimeoutWebInner web 工具超时同样要计入
func TestEffectiveToolTimeoutWebInner(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Web.Timeout = 45 * time.Second

	if got := effectiveToolTimeout(cfg); got <= cfg.Tools.Web.Timeout {
		t.Fatalf("外层 %v 必须大于 web 超时 %v", got, cfg.Tools.Web.Timeout)
	}
}

// TestEffectiveMessageTimeout 整轮超时默认 10 分钟，可被配置覆盖
func TestEffectiveMessageTimeout(t *testing.T) {
	if got := effectiveMessageTimeout(&config.Config{}); got != goroutineTimeout {
		t.Fatalf("默认应为 %v，实际 %v", goroutineTimeout, got)
	}

	cfg := &config.Config{}
	cfg.Agent.MessageTimeout = 30 * time.Minute
	if got := effectiveMessageTimeout(cfg); got != 30*time.Minute {
		t.Fatalf("配置应生效，实际 %v", got)
	}
}
