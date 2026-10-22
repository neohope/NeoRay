package memory

import (
	"fmt"
	"strings"
	"testing"
)

// TestBuildHistoryTextWithinBudget 历史文本总量必须受预算约束
func TestBuildHistoryTextWithinBudget(t *testing.T) {
	d := &Dream{}
	entries := make([]HistoryEntry, 0, 20)
	for i := 0; i < 20; i++ {
		entries = append(entries, HistoryEntry{
			Timestamp: fmt.Sprintf("2026-10-22T10:%02d:00+08:00", i%60),
			Content:   strings.Repeat("x", HistoryEntryPreviewMaxChars), // 每条都顶满
			Cursor:    i,
		})
	}

	out := d.buildHistoryText(entries)
	if len(out) > dreamHistoryBudgetChars+200 {
		t.Fatalf("历史文本超预算: %d > %d", len(out), dreamHistoryBudgetChars)
	}
	if !strings.Contains(out, "已省略") {
		t.Fatal("被省略的条目应有说明")
	}
}

// TestBuildHistoryTextKeepsNewest 预算不足时应优先保留最新条目
func TestBuildHistoryTextKeepsNewest(t *testing.T) {
	d := &Dream{}
	entries := make([]HistoryEntry, 0, 20)
	for i := 0; i < 20; i++ {
		prefix := fmt.Sprintf("ENTRY-%02d-", i)
		entries = append(entries, HistoryEntry{
			Timestamp: "2026-10-22T10:00:00+08:00",
			Content:   prefix + strings.Repeat("x", HistoryEntryPreviewMaxChars),
			Cursor:    i,
		})
	}

	out := d.buildHistoryText(entries)
	if !strings.Contains(out, "ENTRY-19-") {
		t.Fatal("最新条目必须保留")
	}
	if strings.Contains(out, "ENTRY-00-") {
		t.Fatal("最旧的条目应被省略（预算不足时）")
	}
}

// TestBuildHistoryTextSmallInputKeepsAll 小输入不应触发省略
func TestBuildHistoryTextSmallInputKeepsAll(t *testing.T) {
	d := &Dream{}
	entries := []HistoryEntry{
		{Timestamp: "2026-10-21T10:00:00+08:00", Content: "第一条"},
		{Timestamp: "2026-10-22T10:00:00+08:00", Content: "第二条"},
	}
	out := d.buildHistoryText(entries)
	if strings.Contains(out, "已省略") {
		t.Fatalf("小输入不应省略: %s", out)
	}
	for _, want := range []string{"第一条", "第二条"} {
		if !strings.Contains(out, want) {
			t.Fatalf("缺少 %q: %s", want, out)
		}
	}
}
