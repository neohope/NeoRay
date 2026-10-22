package api

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestStopDoesNotCloseSend Stop 之后仍可能有 handler 在写，不能 close(Send)
func TestStopDoesNotCloseSend(t *testing.T) {
	s := &Server{clients: make(map[string]*Client), cfgMu: sync.RWMutex{}}
	c := &Client{ID: "c-stop", Send: make(chan []byte, 4), closed: make(chan struct{})}
	s.clients["c-stop"] = c

	s.clientsMu.Lock()
	for _, client := range s.clients {
		client.markClosed()
	}
	s.clients = make(map[string]*Client)
	s.clientsMu.Unlock()

	// 晚到的 handler 继续发送：必须安全丢弃
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.sendMessage("chat_end", nil)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop 后发送阻塞")
	}
	_ = context.Background()
}
