package session

import "testing"

func msgs(ids ...string) []Message {
	out := make([]Message, 0, len(ids))
	for _, id := range ids {
		out = append(out, Message{ID: id, Role: "user", Content: id})
	}
	return out
}

// TestPaginateTail 不传游标时返回最新 limit 条
func TestPaginateTail(t *testing.T) {
	all := msgs("m1", "m2", "m3", "m4", "m5")
	page, hasMore, next := PaginateMessages(all, 2, 0)
	if len(page) != 2 || page[0].ID != "m4" || page[1].ID != "m5" {
		t.Fatalf("应返回 m4,m5，实际 %+v", page)
	}
	if !hasMore || next != 3 {
		t.Fatalf("hasMore/next 不对: %v %d", hasMore, next)
	}
}

// TestPaginateBefore 传游标时返回更早的一页
func TestPaginateBefore(t *testing.T) {
	all := msgs("m1", "m2", "m3", "m4", "m5")
	page, hasMore, next := PaginateMessages(all, 2, 3)
	if len(page) != 2 || page[0].ID != "m2" || page[1].ID != "m3" {
		t.Fatalf("应返回 m2,m3，实际 %+v", page)
	}
	if !hasMore || next != 1 {
		t.Fatalf("hasMore/next 不对: %v %d", hasMore, next)
	}
}

// TestPaginateNoMore 到底时 hasMore=false
func TestPaginateNoMore(t *testing.T) {
	page, hasMore, next := PaginateMessages(msgs("m1", "m2"), 10, 0)
	if len(page) != 2 || hasMore || next != -1 {
		t.Fatalf("应返回全部且无更多: %d %v %d", len(page), hasMore, next)
	}
}

// TestPaginateWithoutMessageIDs 历史上存在 id 为空的消息，索引游标仍应可用
func TestPaginateWithoutMessageIDs(t *testing.T) {
	all := []Message{{Role: "user"}, {Role: "assistant"}, {Role: "tool"}}
	page, hasMore, next := PaginateMessages(all, 1, 0)
	if len(page) != 1 || page[0].Role != "tool" || !hasMore || next != 2 {
		t.Fatalf("索引分页应支持无 ID 消息: %d %v %d", len(page), hasMore, next)
	}
}

// TestPaginateEdgeCases 空列表与越界游标
func TestPaginateEdgeCases(t *testing.T) {
	if page, more, next := PaginateMessages(nil, 5, 0); page != nil || more || next != -1 {
		t.Fatal("空列表应返回空")
	}
	all := msgs("m1", "m2")
	if page, _, _ := PaginateMessages(all, 0, 0); len(page) != 2 {
		t.Fatalf("limit=0 应返回全部，实际 %d", len(page))
	}
	// 越界游标按末尾处理，避免返回空页
	if page, _, _ := PaginateMessages(all, 5, 999); len(page) != 2 {
		t.Fatalf("越界游标应回退到末尾，实际 %d", len(page))
	}
	if page, _, _ := PaginateMessages(all, 5, 1); len(page) != 1 {
		t.Fatalf("before=1 应只返回 m1，实际 %d", len(page))
	}
}
