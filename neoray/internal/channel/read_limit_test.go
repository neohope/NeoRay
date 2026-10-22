package channel

import (
	"strings"
	"testing"
)

// TestReadLimitedBodyRejectsOversize 超过上限必须报错，而不是返回半截内容
func TestReadLimitedBodyRejectsOversize(t *testing.T) {
	body := strings.NewReader(strings.Repeat("x", 2000))
	if _, err := readLimitedBody(body, 1000); err == nil {
		t.Fatal("超过上限应报错（静默截断会让半截文件落盘）")
	}
}

// TestReadLimitedBodyAllowsExact 刚好等于上限应放行
func TestReadLimitedBodyAllowsExact(t *testing.T) {
	data := strings.Repeat("y", 1000)
	got, err := readLimitedBody(strings.NewReader(data), 1000)
	if err != nil {
		t.Fatalf("等于上限不应报错: %v", err)
	}
	if len(got) != 1000 {
		t.Fatalf("内容长度不对: %d", len(got))
	}
}

// TestReadLimitedBodySmall 正常小响应不受影响
func TestReadLimitedBodySmall(t *testing.T) {
	got, err := readLimitedBody(strings.NewReader("hello"), 1000)
	if err != nil || string(got) != "hello" {
		t.Fatalf("小响应处理异常: %q err=%v", got, err)
	}
}
