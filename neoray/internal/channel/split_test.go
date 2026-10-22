package channel

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestSplitShortContentUnchanged 短消息不应被切分
func TestSplitShortContentUnchanged(t *testing.T) {
	in := "短回复，不用切分。"
	got := splitMessageContent(in, 100)
	if len(got) != 1 || got[0] != in {
		t.Fatalf("短内容不应切分: %v", got)
	}
}

// TestSplitLongContentAllPartsWithinLimit 切分后每段都不超限，且内容不丢
func TestSplitLongContentAllPartsWithinLimit(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		sb.WriteString("这是一段比较长的回复内容，用来验证切分逻辑。\n\n")
	}
	in := sb.String()

	const limit = 500
	parts := splitMessageContent(in, limit)
	if len(parts) < 2 {
		t.Fatalf("长内容应被切分，实际 %d 段", len(parts))
	}
	for i, p := range parts {
		if len(p) > limit {
			t.Fatalf("第 %d 段超限: %d > %d", i, len(p), limit)
		}
		if strings.TrimSpace(p) == "" {
			t.Fatalf("第 %d 段为空", i)
		}
	}
	// 内容不应丢失（忽略段间被去掉的换行）
	joined := strings.Join(parts, "")
	if len(joined) < len(strings.TrimSpace(in))-len(parts)*2 {
		t.Fatalf("切分后内容明显变少: %d → %d", len(in), len(joined))
	}
}

// TestSplitPrefersParagraphBoundary 应优先在段落边界断开
func TestSplitPrefersParagraphBoundary(t *testing.T) {
	para := strings.Repeat("a", 300)
	in := para + "\n\n" + para + "\n\n" + para
	parts := splitMessageContent(in, 400)
	if len(parts) < 2 {
		t.Fatalf("应切分为多段，实际 %d", len(parts))
	}
	for i, p := range parts {
		if strings.HasPrefix(p, "\n") {
			t.Fatalf("第 %d 段以换行开头（切分点不对）", i)
		}
	}
}

// TestSplitHardCutKeepsUTF8 超长单行硬切时不能切出半个汉字
func TestSplitHardCutKeepsUTF8(t *testing.T) {
	in := strings.Repeat("中文", 1000) // 6000 字节，无换行
	parts := splitMessageContent(in, 500)
	for i, p := range parts {
		if !utf8.ValidString(p) {
			t.Fatalf("第 %d 段产生非法 UTF-8", i)
		}
	}
}

// TestSplitEmpty 空内容返回 nil
func TestSplitEmpty(t *testing.T) {
	if got := splitMessageContent("   \n\n ", 100); got != nil {
		t.Fatalf("空内容应返回 nil，实际 %v", got)
	}
}
