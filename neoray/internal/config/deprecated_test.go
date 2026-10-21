package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestDetectDeprecatedKeys 老配置里残留的已移除段落应被识别
func TestDetectDeprecatedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "neoray.toml")
	content := `
[database]
driver = "sqlite"

[channels.websocket]
enabled = true

[security.upload]
max_size = 1024

[tools.shell]
allowed_commands = ["ls"]
blocked_commands = ["rm"]
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("读取测试配置失败: %v", err)
	}

	keys := detectDeprecatedKeys(v)

	want := map[string]bool{
		"database":                     true,
		"channels.websocket":           true,
		"security.upload":              true,
		"tools.shell.allowed_commands": true,
	}
	for w := range want {
		found := false
		for _, k := range keys {
			if k == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("未识别到已移除配置 %q，实际: %v", w, keys)
		}
	}
	for _, k := range keys {
		if k == "tools.shell.blocked_commands" {
			t.Fatalf("仍在生效的配置被误判: %v", keys)
		}
	}
}

// TestUnusedConfigWarningsFormat 告警里要写清「已移除 + 原因」
func TestUnusedConfigWarningsFormat(t *testing.T) {
	cfg := &Config{DeprecatedKeys: []string{"database"}}
	warns := UnusedConfigWarnings(cfg)
	if len(warns) != 1 {
		t.Fatalf("应有 1 条告警，实际 %v", warns)
	}
	if !strings.Contains(warns[0], "database") || !strings.Contains(warns[0], "已移除") {
		t.Fatalf("告警内容不完整: %s", warns[0])
	}
}

// TestUnusedConfigWarningsEmpty 没有残留配置时不打扰用户
func TestUnusedConfigWarningsEmpty(t *testing.T) {
	if warns := UnusedConfigWarnings(&Config{}); len(warns) != 0 {
		t.Fatalf("空配置不应告警: %v", warns)
	}
}
