package channel

import (
	"testing"
	"time"
)

// TestTokenRefreshMarginScalesWithTTL 短有效期不应触发"立刻刷新"风暴
func TestTokenRefreshMarginScalesWithTTL(t *testing.T) {
	// 有效期 20 分钟 → 提前 2 分钟刷新
	if got := tokenRefreshMargin(time.Now().Add(20 * time.Minute)); got < 100*time.Second || got > 130*time.Second {
		t.Fatalf("20 分钟有效期的提前量应约 2 分钟，实际 %v", got)
	}

	// 有效期 2 小时 → 夹在上限 10 分钟
	if got := tokenRefreshMargin(time.Now().Add(2 * time.Hour)); got != 10*time.Minute {
		t.Fatalf("长有效期应夹在 10 分钟，实际 %v", got)
	}

	// 有效期 30 秒 → 夹在下限 1 分钟（宁可提前也不要漏刷）
	if got := tokenRefreshMargin(time.Now().Add(30 * time.Second)); got != time.Minute {
		t.Fatalf("短有效期应夹在 1 分钟，实际 %v", got)
	}

	// 已过期
	if got := tokenRefreshMargin(time.Now().Add(-time.Minute)); got != 0 {
		t.Fatalf("已过期应返回 0，实际 %v", got)
	}
}
