package api

import (
	"testing"
	"time"
)

// TestRateLimiterRefillsFractionally 小间隔请求应逐步补回令牌。
// 回归：整数除法丢掉零头却推进 lastSeen，每 900ms 一次的客户端会永久 429。
func TestRateLimiterRefillsFractionally(t *testing.T) {
	rl := newRateLimiter(60, 5, time.Minute) // 每分钟 60 次，桶容量 5
	defer rl.stop()

	ip := "1.2.3.4"
	// 先耗尽突发额度
	for i := 0; i < 5; i++ {
		if !rl.allow(ip) {
			t.Fatalf("前 5 次应放行，第 %d 次被拒", i+1)
		}
	}
	if rl.allow(ip) {
		t.Fatal("额度耗尽后应拒绝")
	}

	// 每次推进 1 秒 = 1 个令牌（60/分钟），连续 3 秒应恢复 3 次
	allowed := 0
	for i := 0; i < 3; i++ {
		time.Sleep(1050 * time.Millisecond)
		if rl.allow(ip) {
			allowed++
		}
	}
	if allowed < 2 {
		t.Fatalf("每秒应有约 1 个令牌补充，3 秒内实际只放行 %d 次（余量被丢弃）", allowed)
	}
}

// TestRateLimiterBurstThenWindow 桶容量应限制瞬时突发
func TestRateLimiterBurstThenWindow(t *testing.T) {
	rl := newRateLimiter(600, 3, time.Minute)
	defer rl.stop()

	ip := "5.6.7.8"
	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.allow(ip) {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("容量 3 时应只放行 3 次，实际 %d", allowed)
	}
}
