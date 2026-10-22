package channel

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestSendWithRetrySucceedsAfterFailure 首次失败后应重试成功
func TestSendWithRetrySucceedsAfterFailure(t *testing.T) {
	attempts := 0
	err := sendWithRetry(context.Background(), "feishu", func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("重试后应成功，实际 %v", err)
	}
	if attempts != 2 {
		t.Fatalf("应调用 2 次，实际 %d", attempts)
	}
}

// TestSendWithRetryGivesUp 连续失败应在 3 次后放弃并返回最后错误
func TestSendWithRetryGivesUp(t *testing.T) {
	attempts := 0
	err := sendWithRetry(context.Background(), "feishu", func(ctx context.Context) error {
		attempts++
		return errors.New("still broken")
	})
	if err == nil {
		t.Fatal("应返回错误")
	}
	if attempts != sendMaxAttempts {
		t.Fatalf("应尝试 %d 次，实际 %d", sendMaxAttempts, attempts)
	}
}

// TestSendWithRetryRespectsContext ctx 取消后不应继续重试
func TestSendWithRetryRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := sendWithRetry(ctx, "feishu", func(ctx context.Context) error {
		attempts++
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回 ctx 错误，实际 %v", err)
	}
	if attempts > 2 {
		t.Fatalf("取消后不应继续重试，实际尝试 %d 次", attempts)
	}
}
