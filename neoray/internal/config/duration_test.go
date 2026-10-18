package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDurationParsing 验证 duration 配置的两种写法都能正确解析：
//   - 裸整数（按「秒」解释，修复前会被误当作纳秒）
//   - 带单位字符串
func TestDurationParsing(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("NEORAY_HOME", dir)
	defer os.Unsetenv("NEORAY_HOME")

	cfgPath := filepath.Join(dir, "neoray.toml")
	content := `
[server]
host = "127.0.0.1"
port = 18099
read_timeout = 60
write_timeout = "90s"
idle_timeout = 120

[security.auth]
jwt_expiry = 86400

[channels.websocket]
ping_interval = 30
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"整数 read_timeout=60 → 60s", cfg.Server.ReadTimeout, 60 * time.Second},
		{"字符串 write_timeout=90s", cfg.Server.WriteTimeout, 90 * time.Second},
		{"整数 idle_timeout=120 → 120s", cfg.Server.IdleTimeout, 120 * time.Second},
		{"整数 jwt_expiry=86400 → 24h", cfg.Security.Auth.JWTExpiry, 24 * time.Hour},
		{"整数 ping_interval=30 → 30s", cfg.Channels.WebSocket.PingInterval, 30 * time.Second},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("FAIL %s: got %v, want %v", c.name, c.got, c.want)
		} else {
			t.Logf("PASS %s: %v", c.name, c.got)
		}
	}
}
