package agent

import (
	"context"
	"testing"
	"time"
)

// TestSessionLockAcquireTimeout 锁被占用时，超时应返回 false 而不是永久阻塞
func TestSessionLockAcquireTimeout(t *testing.T) {
	entry := &sessionLockEntry{}
	entry.mu.Lock() // 模拟「上一轮还在处理」

	start := time.Now()
	ok := entry.acquire(context.Background(), 120*time.Millisecond)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("锁被占用时不应获取成功")
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("应等待到超时，实际只等了 %v", elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("超时控制失效，等了 %v", elapsed)
	}

	entry.mu.Unlock()
}

// TestSessionLockAcquireSucceeds 空闲锁应能立即获取
func TestSessionLockAcquireSucceeds(t *testing.T) {
	entry := &sessionLockEntry{}
	if !entry.acquire(context.Background(), time.Second) {
		t.Fatal("空闲锁应获取成功")
	}
	entry.mu.Unlock()
}

// TestSessionLockAcquireRespectsContext ctx 取消时应立即返回
func TestSessionLockAcquireRespectsContext(t *testing.T) {
	entry := &sessionLockEntry{}
	entry.mu.Lock()
	defer entry.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	ok := entry.acquire(ctx, 10*time.Second)
	if ok {
		t.Fatal("ctx 取消后不应获取成功")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("ctx 取消未生效，等了 %v", time.Since(start))
	}
}

// TestLockWaitTimeoutExceedsTurn 锁等待上限必须大于单轮处理时间
func TestLockWaitTimeoutExceedsTurn(t *testing.T) {
	al := &AgentLoop{messageTimeout: 10 * time.Minute}
	if got := al.lockWaitTimeout(); got <= al.messageTimeout {
		t.Fatalf("锁等待 %v 应大于单轮超时 %v，否则正常长任务会被误判卡死", got, al.messageTimeout)
	}
}
