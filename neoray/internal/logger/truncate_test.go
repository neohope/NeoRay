package logger

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateKeepsUTF8 截断不能切出半个汉字
func TestTruncateKeepsUTF8(t *testing.T) {
	in := strings.Repeat("中文内容", 500)
	got := Truncate(in, 101)
	if !utf8.ValidString(got) {
		t.Fatal("截断后产生非法 UTF-8")
	}
	if len(got) > 140 {
		t.Fatalf("截断后仍过长: %d", len(got))
	}
}

// TestTruncateShortUnchanged 短文本原样返回
func TestTruncateShortUnchanged(t *testing.T) {
	in := "hello"
	if got := Truncate(in, 100); got != in {
		t.Fatalf("短文本不应改动: %q", got)
	}
}

// TestTruncateZeroLimit 非正数上限原样返回
func TestTruncateZeroLimit(t *testing.T) {
	in := "hello world"
	if got := Truncate(in, 0); got != in {
		t.Fatalf("0 上限应原样返回: %q", got)
	}
}
