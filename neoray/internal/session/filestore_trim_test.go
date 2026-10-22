package session

import (
	"testing"
)

// TestFileStoreTrimsMessages 文件存储必须按上限裁剪，否则会话文件无限增长
func TestFileStoreTrimsMessages(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, WithFileStoreMaxMessages(5))
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	sess := NewSession("feishu", "u1")
	sess.ID = "s1"
	for i := 0; i < 20; i++ {
		sess.Messages = append(sess.Messages, Message{
			Role:    "user",
			Content: "m",
		})
	}
	if err := store.Save(sess); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	got, err := store.Get("s1")
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if len(got.Messages) != 5 {
		t.Fatalf("应裁剪到 5 条，实际 %d 条（默认文件存储此前完全不裁剪）", len(got.Messages))
	}

	// 落盘文件也应是裁剪后的
	onDisk, err := store.Get("s1")
	if err != nil {
		t.Fatalf("再次读取失败: %v", err)
	}
	if len(onDisk.Messages) != 5 {
		t.Fatalf("落盘会话应同样裁剪，实际 %d", len(onDisk.Messages))
	}
}

// TestTrimMessagesKeepsNewest 裁剪应保留最新消息
func TestTrimMessagesKeepsNewest(t *testing.T) {
	msgs := []Message{{Content: "a"}, {Content: "b"}, {Content: "c"}, {Content: "d"}}
	got := trimMessages(msgs, 2)
	if len(got) != 2 || got[0].Content != "c" || got[1].Content != "d" {
		t.Fatalf("应保留最新的 c,d，实际 %+v", got)
	}

	if out := trimMessages(msgs, 0); len(out) != 4 {
		t.Fatalf("0 表示不限制，实际 %d", len(out))
	}
	if out := trimMessages(msgs, 10); len(out) != 4 {
		t.Fatalf("未超限时不应改动，实际 %d", len(out))
	}
}
