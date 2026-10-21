package config

import "testing"

// TestUnusedConfigWarningsFlagsPhantomSections 配置了但不生效的段落要被告警
func TestUnusedConfigWarningsFlagsPhantomSections(t *testing.T) {
	cfg := &Config{}
	cfg.Database.Driver = "postgres"
	cfg.Database.Postgres.Host = "127.0.0.1"
	cfg.Security.Upload.MaxSize = 1024
	cfg.Tools.Cron.MaxJobs = 5 // 已接线，不应告警

	warns := UnusedConfigWarnings(cfg)
	if len(warns) < 2 {
		t.Fatalf("应至少告警 database 与 security.upload，实际: %v", warns)
	}

	joined := ""
	for _, w := range warns {
		joined += w + "\n"
	}
	if !contains(joined, "[database]") {
		t.Fatalf("缺少 database 告警: %v", warns)
	}
	if !contains(joined, "[security.upload]") {
		t.Fatalf("缺少 upload 告警: %v", warns)
	}
	if contains(joined, "cron") {
		t.Fatalf("已接线的 cron.max_jobs 不应被告警: %v", warns)
	}
}

// TestUnusedConfigWarningsQuietByDefault 默认配置不应产生噪音
func TestUnusedConfigWarningsQuietByDefault(t *testing.T) {
	cfg := &Config{}
	if warns := UnusedConfigWarnings(cfg); len(warns) != 0 {
		t.Fatalf("空配置不应告警，实际: %v", warns)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
