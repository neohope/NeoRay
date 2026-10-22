package tools

import (
	"strings"
	"testing"
)

// TestCappedBufferStopsGrowing 超过上限后不再增长
func TestCappedBufferStopsGrowing(t *testing.T) {
	b := newCappedBuffer(100)
	chunk := strings.Repeat("x", 60)
	for i := 0; i < 100; i++ {
		if _, err := b.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.String()) != 100 {
		t.Fatalf("缓冲应停在上限 100，实际 %d", len(b.String()))
	}
	if b.Dropped() != 60*100-100 {
		t.Fatalf("丢弃计数不对: %d", b.Dropped())
	}
}

// TestCappedBufferSmallInput 未超限时不丢内容
func TestCappedBufferSmallInput(t *testing.T) {
	b := newCappedBuffer(100)
	b.Write([]byte("hello"))
	if b.String() != "hello" {
		t.Fatalf("内容不对: %q", b.String())
	}
	if b.Dropped() != 0 {
		t.Fatalf("不应有丢弃: %d", b.Dropped())
	}
}

// TestCappedBufferSplitAtBoundary 单次写入跨越上限时正确截断
func TestCappedBufferSplitAtBoundary(t *testing.T) {
	b := newCappedBuffer(10)
	n, err := b.Write([]byte("0123456789ABCDEF"))
	if err != nil || n != 16 {
		t.Fatalf("应报告写满 %d 字节，实际 %d err=%v", 16, n, err)
	}
	if b.String() != "0123456789" {
		t.Fatalf("截断结果不对: %q", b.String())
	}
	if b.Dropped() != 6 {
		t.Fatalf("丢弃数不对: %d", b.Dropped())
	}
}
