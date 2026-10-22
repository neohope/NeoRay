package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateToolResultShort 短输出原样返回
func TestTruncateToolResultShort(t *testing.T) {
	in := "hello world"
	if got := TruncateToolResult(in, 100); got != in {
		t.Fatalf("短输出不应改动: %q", got)
	}
}

// TestTruncateToolResultLong 长输出应被截断且保留头尾
func TestTruncateToolResultLong(t *testing.T) {
	head := strings.Repeat("A", 1000)
	mid := strings.Repeat("M", 100000)
	tail := strings.Repeat("Z", 1000)
	in := head + mid + tail

	got := TruncateToolResult(in, 4000)

	if len(got) > 4200 {
		t.Fatalf("截断后仍过长: %d", len(got))
	}
	if !strings.HasPrefix(got, "AAA") {
		t.Fatal("应保留头部")
	}
	if !strings.HasSuffix(got, "ZZZ") {
		t.Fatal("应保留尾部（错误信息/汇总常在末尾）")
	}
	if !strings.Contains(got, "已截断") {
		t.Fatal("应包含截断提示")
	}
	if strings.Count(got, "M") > 5000 {
		t.Fatalf("中间部分未被丢弃: %d 个 M", strings.Count(got, "M"))
	}
}

// TestTruncateToolResultNoLimit 0 表示不限制
func TestTruncateToolResultNoLimit(t *testing.T) {
	in := strings.Repeat("x", 50000)
	if got := TruncateToolResult(in, 0); got != in {
		t.Fatalf("0 应表示不限制，实际长度 %d", len(got))
	}
}

// TestTruncateToolResultKeepsUTF8 不能把多字节字符切成半个
func TestTruncateToolResultKeepsUTF8(t *testing.T) {
	in := strings.Repeat("中文测试", 5000) // 每个字符 3 字节
	got := TruncateToolResult(in, 1001)
	if !utf8.ValidString(got) {
		t.Fatal("截断后产生非法 UTF-8（会把乱码发给模型）")
	}
}

// TestTruncateToolResultTinyBudget 上限极小时不应 panic
func TestTruncateToolResultTinyBudget(t *testing.T) {
	in := strings.Repeat("y", 1000)
	for _, max := range []int{1, 10, 60, 100} {
		got := TruncateToolResult(in, max)
		if len(got) == 0 {
			t.Fatalf("max=%d 时不应返回空", max)
		}
	}
}
