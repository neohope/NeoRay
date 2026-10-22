package bus

import (
	"regexp"
	"testing"
)

// TestGenerateIDFormat 正常路径应为 32 位十六进制（128 位熵）
func TestGenerateIDFormat(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for i := 0; i < 100; i++ {
		id := generateID()
		if !re.MatchString(id) {
			t.Fatalf("ID 格式不对: %q（长度 %d）", id, len(id))
		}
	}
}

// TestGenerateIDUnique 连续生成不应重复
func TestGenerateIDUnique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := generateID()
		if seen[id] {
			t.Fatalf("ID 重复: %s", id)
		}
		seen[id] = true
	}
}
