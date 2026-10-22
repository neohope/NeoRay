package bus

import (
	"testing"
	"time"
)

// TestSubscriberBufferConstant 订阅缓冲应足够大（此前 50，突发即满）
func TestSubscriberBufferConstant(t *testing.T) {
	if OutboundSubscriberBuffer < 256 {
		t.Fatalf("订阅缓冲应 >= 256，实际 %d", OutboundSubscriberBuffer)
	}
}

// TestDispatchDeliversWhenSpaceAvailable 有空间时应正常投递且不丢弃
func TestDispatchDeliversWhenSpaceAvailable(t *testing.T) {
	b := NewMessageBus(8, 8)
	out := make(chan *OutboundMessage, 4)
	if err := b.SubscribeOutbound("test", out); err != nil {
		t.Fatalf("订阅失败: %v", err)
	}
	if err := b.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer b.Stop()

	if err := b.PublishOutbound(&OutboundMessage{ID: "m1"}); err != nil {
		t.Fatalf("发布失败: %v", err)
	}

	select {
	case msg := <-out:
		if msg.ID != "m1" {
			t.Fatalf("收到错误消息: %s", msg.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("消息未投递")
	}

	if got := b.GetDropMetrics()["test"]; got != 0 {
		t.Fatalf("不应有丢弃，实际 %d", got)
	}
}

// TestFullSubscriberBackpressuresThenDrops 缓冲满时先等待宽限期，再丢弃。
// 之前是立刻 default 丢弃：突发几条回复就会丢消息。
func TestFullSubscriberBackpressuresThenDrops(t *testing.T) {
	old := outboundBackpressureGrace
	outboundBackpressureGrace = 150 * time.Millisecond
	defer func() { outboundBackpressureGrace = old }()

	b := NewMessageBus(8, 8)
	out := make(chan *OutboundMessage, 1) // 不消费，容量 1
	if err := b.SubscribeOutbound("slow", out); err != nil {
		t.Fatalf("订阅失败: %v", err)
	}
	if err := b.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer b.Stop()

	// 第一条占满缓冲，后续每条都要等宽限期才丢弃
	for i := 0; i < 3; i++ {
		if err := b.PublishOutbound(&OutboundMessage{ID: "m"}); err != nil {
			t.Fatalf("发布失败: %v", err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b.GetDropMetrics()["slow"] >= 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("缓冲满后应在宽限期结束时计入丢弃，实际指标: %v", b.GetDropMetrics())
}

// TestDropWarningAggregated 丢弃告警应聚合（lastWarn 只更新一次/分钟）
func TestDropWarningAggregated(t *testing.T) {
	b := NewMessageBus(8, 8)
	b.recordDrop("sub")
	b.warnDrop("sub", "id1")
	if got := b.drops.lastWarn["sub"]; got.IsZero() {
		t.Fatal("首次告警应记录时间")
	}
	before := b.drops.lastWarn["sub"]
	b.warnDrop("sub", "id2") // 一分钟内不应更新
	if got := b.drops.lastWarn["sub"]; !got.Equal(before) {
		t.Fatalf("告警未聚合，时间被更新: %v -> %v", before, got)
	}
}
