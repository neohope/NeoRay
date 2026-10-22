package channel

import (
	"testing"
	"time"
)

// TestTokenRefreshMarginScalesWithTTL 提前量按「总有效期」的比例计算
func TestTokenRefreshMarginScalesWithTTL(t *testing.T) {
	// 总有效期 20 分钟 → 提前 2 分钟
	if got := tokenRefreshMarginForTTL(20 * time.Minute); got != 2*time.Minute {
		t.Fatalf("20 分钟有效期应提前 2 分钟，实际 %v", got)
	}
	// 总有效期 2 小时 → 夹在上限 10 分钟
	if got := tokenRefreshMarginForTTL(2 * time.Hour); got != 10*time.Minute {
		t.Fatalf("长有效期应夹在 10 分钟，实际 %v", got)
	}
	// 总有效期 30 秒 → 夹在下限 1 分钟
	if got := tokenRefreshMarginForTTL(30 * time.Second); got != time.Minute {
		t.Fatalf("短有效期应夹在 1 分钟，实际 %v", got)
	}
	if got := tokenRefreshMarginForTTL(0); got != 0 {
		t.Fatalf("0 应返回 0，实际 %v", got)
	}
}

// TestTokenNeedsRefreshUsesTotalTTL 关键回归：token 快到期时必须判定需要刷新。
// 旧实现用剩余有效期算 margin，判定恒不成立，token 过期后要等 ticker 才刷新。
func TestTokenNeedsRefreshUsesTotalTTL(t *testing.T) {
	ch := &FeishuChannel{}

	// 没有任何 token → 需要刷新
	if !ch.tokenNeedsRefresh() {
		t.Fatal("无 token 时应需要刷新")
	}

	// 2 小时有效期、刚拿到的 token：不该刷新
	ch.tenantToken = "t"
	ch.tokenTTL = 2 * time.Hour
	ch.tokenExpiry = time.Now().Add(2 * time.Hour)
	if ch.tokenNeedsRefresh() {
		t.Fatal("刚拿到的长有效期 token 不应刷新")
	}

	// 同一 token 剩余 5 分钟（margin 10 分钟）→ 应刷新
	ch.tokenExpiry = time.Now().Add(5 * time.Minute)
	if !ch.tokenNeedsRefresh() {
		t.Fatal("剩余时间小于提前量时应刷新")
	}

	// 已过期 → 必须刷新
	ch.tokenExpiry = time.Now().Add(-time.Second)
	if !ch.tokenNeedsRefresh() {
		t.Fatal("过期 token 必须刷新")
	}
}
