package api

import (
	"sync"
	"testing"
	"time"
)

// TestSendMessageAfterClose 连接注销后写消息不能 panic（之前会 send on closed channel）
func TestSendMessageAfterClose(t *testing.T) {
	s := &Server{clients: make(map[string]*Client), cfgMu: sync.RWMutex{}}
	c := &Client{
		ID:     "c1",
		Send:   make(chan []byte, 4),
		Server: s,
		closed: make(chan struct{}),
	}
	s.clients["c1"] = c

	s.removeClient("c1")

	// 注销后 handler goroutine 仍可能尝试发送：必须安全丢弃，而不是 panic
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.sendMessage("chat_end", map[string]string{"x": "y"})
		c.sendError("timeout", "late")
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sendMessage 在关闭后阻塞了")
	}
}

// TestRemoveClientIdempotent 重复注销不应 panic
func TestRemoveClientIdempotent(t *testing.T) {
	s := &Server{clients: make(map[string]*Client), cfgMu: sync.RWMutex{}}
	c := &Client{ID: "c2", Send: make(chan []byte, 1), Server: s, closed: make(chan struct{})}
	s.clients["c2"] = c

	s.removeClient("c2")
	s.removeClient("c2") // 不应 panic
}

// TestSendMessageBeforeClose 正常发送仍然生效
func TestSendMessageBeforeClose(t *testing.T) {
	s := &Server{clients: make(map[string]*Client), cfgMu: sync.RWMutex{}}
	c := &Client{ID: "c3", Send: make(chan []byte, 4), Server: s, closed: make(chan struct{})}

	c.sendMessage("chat_start", map[string]string{"a": "b"})
	select {
	case msg := <-c.Send:
		if len(msg) == 0 {
			t.Fatal("发送内容为空")
		}
	default:
		t.Fatal("消息没有进入发送队列")
	}
}
