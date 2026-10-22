package channel

import (
	"context"
	"time"

	"neoray/internal/logger"
)

// sendRetryDelay 首次重试前的等待；第二次及以后按倍数增长
const (
	sendMaxAttempts = 3
	sendBaseDelay   = 300 * time.Millisecond
)

// sendWithRetry 执行发送函数，失败时按指数退避重试。
//
// 之前频道发送失败只记一条 logger.Error 就放弃：飞书侧一次网络抖动
// 或 5xx 就等于「这条回复永远丢了」，用户只看到"AI 没回"。
// 这里做有限重试（最多 3 次，300ms/600ms 退避），并对 ctx 取消立即返回。
func sendWithRetry(ctx context.Context, channelName string, fn func(context.Context) error) error {
	var lastErr error
	delay := sendBaseDelay

	for attempt := 1; attempt <= sendMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == sendMaxAttempts {
			break
		}

		logger.Warn("Channel send failed, retrying",
			logger.String("channel", channelName),
			logger.Int("attempt", attempt),
			logger.Duration("delay", delay),
			logger.ErrorField(err))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
	}

	return lastErr
}
