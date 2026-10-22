package session

import (
	"sync"
	"testing"
	"time"
)

// TestDeepCopyIsolatesNestedFields 拷贝后修改嵌套结构不能影响原会话
func TestDeepCopyIsolatesNestedFields(t *testing.T) {
	s := NewSession("feishu", "u1")
	s.ID = "s1"
	s.Metadata = map[string]any{"k": "v", "nested": map[string]any{"a": 1}, "list": []any{"x"}}
	s.AddMessage(Message{
		Role:        "assistant",
		Content:     "hi",
		ToolCalls:   []ToolCall{{ID: "c1", Name: "shell", Arguments: `{"command":"ls"}`}},
		ToolResults: []ToolResult{{ToolCallID: "c1", Content: "out"}},
		Metadata:    map[string]any{"trace": map[string]any{"step": 1}},
	})

	cp := s.DeepCopy()

	// 修改拷贝的所有嵌套结构
	cp.Metadata["k"] = "changed"
	cp.Metadata["nested"].(map[string]any)["a"] = 99
	cp.Metadata["list"].([]any)[0] = "changed"
	cp.Messages[0].ToolCalls[0].Name = "hacked"
	cp.Messages[0].ToolResults[0].Content = "hacked"
	cp.Messages[0].Metadata["trace"].(map[string]any)["step"] = 99

	// 原会话必须保持不变
	if s.Metadata["k"] != "v" {
		t.Fatalf("顶层 map 被共享: %v", s.Metadata["k"])
	}
	if s.Metadata["nested"].(map[string]any)["a"] != 1 {
		t.Fatal("嵌套 map 被共享（并发下会 panic: concurrent map read/write）")
	}
	if s.Metadata["list"].([]any)[0] != "x" {
		t.Fatal("嵌套 slice 被共享")
	}
	if s.Messages[0].ToolCalls[0].Name != "shell" {
		t.Fatal("ToolCalls 被共享")
	}
	if s.Messages[0].ToolResults[0].Content != "out" {
		t.Fatal("ToolResults 被共享")
	}
	if s.Messages[0].Metadata["trace"].(map[string]any)["step"] != 1 {
		t.Fatal("Message.Metadata 嵌套被共享")
	}
}

// TestDeepCopyConcurrentMutation 并发下拷贝与写入互不干扰（配合 -race）
func TestDeepCopyConcurrentMutation(t *testing.T) {
	s := NewSession("feishu", "u1")
	s.ID = "s2"
	s.AddMessage(Message{Role: "user", Content: "hi", Metadata: map[string]any{"a": 1}})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp := s.DeepCopy()
			cp.Messages[0].Metadata["a"] = 2
			cp.Metadata = map[string]any{"x": 1}
			cp.AddMessage(Message{Role: "assistant", Content: "ok"})
		}()
	}
	wg.Wait()

	if s.Messages[0].Metadata["a"] != 1 {
		t.Fatal("并发拷贝污染了原会话")
	}
	if len(s.Messages) != 1 {
		t.Fatalf("原会话消息数被改动: %d", len(s.Messages))
	}
}

// TestDeepCopyEmptyFields 空字段不应 panic，且空 map 也必须是独立实例
func TestDeepCopyEmptyFields(t *testing.T) {
	s := NewSession("cli", "u")
	s.ID = "s3"
	s.AddMessage(Message{Role: "user", Content: "x"})
	cp := s.DeepCopy()

	if cp.Metadata == nil {
		t.Fatal("NewSession 初始化的空 map 应被保留")
	}
	cp.Metadata["new"] = 1
	if len(s.Metadata) != 0 {
		t.Fatal("空 map 被共享（并发下会 panic）")
	}
	if cp.Messages[0].ToolCalls != nil || cp.Messages[0].Metadata != nil {
		t.Fatal("nil 嵌套字段应保持 nil")
	}
}

// TestDeepCopySelfReference 自引用/metadata 深度不应无限递归
func TestDeepCopySelfReference(t *testing.T) {
	s := NewSession("cli", "u")
	s.ID = "s4"
	self := map[string]any{}
	self["self"] = self
	s.Metadata = self

	done := make(chan struct{})
	go func() {
		_ = s.DeepCopy()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("自引用数据导致 DeepCopy 卡死")
	}
}
