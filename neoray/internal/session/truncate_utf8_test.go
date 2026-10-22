package session

import (
	"testing"
	"unicode/utf8"
)

// TestTruncateStringKeepsUTF8 截断不能切出半个汉字
func TestTruncateStringKeepsUTF8(t *testing.T) {
	in := "这是一段很长的中文标题内容用于测试"
	for max := 4; max < len(in); max++ {
		got := truncateString(in, max)
		if !utf8.ValidString(got) {
			t.Fatalf("max=%d 切出非法 UTF-8: %q", max, got)
		}
		if len(got) > max+3 { // 后缀占 3 字节
			t.Fatalf("max=%d 结果过长: %d 字节", max, len(got))
		}
	}
}
