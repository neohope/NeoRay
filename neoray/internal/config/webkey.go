package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
)

// EnsureWebAuthKey 在启用 Web UI 但未配置认证密钥时，自动生成一个强随机密钥，
// 写入配置文件并返回明文密钥（供调用方打印一次）。
//
// 设计意图（方案 B）：Web UI 面向本机浏览器使用，要求用户手动生成并填写密钥
// 会显著抬高首次使用门槛。此处做到「开箱即用」，同时不降低安全性——
// 密钥仍是强随机值，只是由程序代生成。
//
// 触发条件（三者全部满足时才动作）：
//  1. web.enabled = true
//  2. security.auth.enabled = true
//  3. security.auth.secret_key 为空
//
// 返回值：
//   - generated: 本次是否新生成
//   - key: 当前生效的密钥（可能是原有值，也可能是新生成值）
//   - err: 持久化失败时非 nil（此时密钥仍已在内存生效，仅无法跨重启保持）
func EnsureWebAuthKey(cfg *Config, configPath string) (bool, string, error) {
	if !cfg.Web.Enabled {
		return false, cfg.Security.Auth.SecretKey, nil
	}
	if !cfg.Security.Auth.Enabled {
		return false, cfg.Security.Auth.SecretKey, nil
	}
	if cfg.Security.Auth.SecretKey != "" {
		return false, cfg.Security.Auth.SecretKey, nil
	}

	key, err := generateSecretKey()
	if err != nil {
		return false, "", fmt.Errorf("generate web auth key: %w", err)
	}

	// 先在内存中生效，保证即使落盘失败本次运行也可用
	cfg.Security.Auth.SecretKey = key

	if configPath == "" {
		return true, key, fmt.Errorf("no config file path available; key is valid for this run only")
	}

	if err := persistSecretKey(configPath, key); err != nil {
		return true, key, fmt.Errorf("persist auth key to %s: %w", configPath, err)
	}

	return true, key, nil
}

// generateSecretKey 生成满足 validateSecretKey 强度要求的随机密钥。
//
// 要求：长度 >= 32，且至少包含大写、小写、数字、特殊字符四类中的三类。
// 这里从每类各取若干字符再打乱，确定性满足要求，避免「随机到不合规」的重试循环。
func generateSecretKey() (string, error) {
	const (
		upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lower   = "abcdefghijklmnopqrstuvwxyz"
		digits  = "0123456789"
		special = "-_"
	)

	// 每类 8 个字符，共 32 个，满足长度下限
	parts := []string{upper, lower, digits, special}
	var sb strings.Builder
	for _, set := range parts {
		for i := 0; i < 8; i++ {
			c, err := randomChar(set)
			if err != nil {
				return "", err
			}
			sb.WriteByte(c)
		}
	}

	// 打乱，避免固定分组顺序（前 8 位必为大写等可预测模式）
	out := []byte(sb.String())
	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

// randomChar 从字符集中随机取一个字符。
func randomChar(set string) (byte, error) {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return set[int(b[0])%len(set)], nil
}

// shuffle 使用 crypto/rand 做 Fisher-Yates 洗牌。
func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		var r [1]byte
		if _, err := rand.Read(r[:]); err != nil {
			return err
		}
		j := int(r[0]) % (i + 1)
		b[i], b[j] = b[j], b[i]
	}
	return nil
}

// persistSecretKey 将密钥写入配置文件的 [security.auth] 段。
//
// 采用行级改写而非整文件重写（viper.WriteConfig 会重新格式化整个文件，
// 破坏用户的注释与排版）。策略：
//   - 段内已有 secret_key 行 → 替换该行
//   - 段存在但无 secret_key 行 → 在段头后插入
//   - 段不存在 → 追加段落到文件末尾
//   - 文件不存在 → 创建最小配置
func persistSecretKey(path, key string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		content := fmt.Sprintf("[security.auth]\nsecret_key = %q\n", key)
		return os.WriteFile(path, []byte(content), 0600)
	}

	const section = "[security.auth]"
	lines := strings.Split(string(data), "\n")

	sectionIdx := -1
	keyIdx := -1
	inSection := false

	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "[") {
			inSection = trimmed == section
			if inSection && sectionIdx == -1 {
				sectionIdx = i
			}
			continue
		}
		if inSection && !strings.HasPrefix(trimmed, "#") {
			if strings.HasPrefix(trimmed, "secret_key") {
				if eq := strings.Index(trimmed, "="); eq != -1 {
					keyIdx = i
					break
				}
			}
		}
	}

	newLine := fmt.Sprintf("secret_key = %q", key)

	switch {
	case keyIdx != -1:
		// 保留原缩进
		indent := lines[keyIdx][:len(lines[keyIdx])-len(strings.TrimLeft(lines[keyIdx], " \t"))]
		lines[keyIdx] = indent + newLine
	case sectionIdx != -1:
		// 段头后插入
		rest := append([]string{}, lines[sectionIdx+1:]...)
		lines = append(lines[:sectionIdx+1], newLine)
		lines = append(lines, rest...)
	default:
		// 追加段落到末尾
		content := strings.TrimRight(string(data), "\n")
		content += "\n\n" + section + "\n" + newLine + "\n"
		return os.WriteFile(path, []byte(content), 0600)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}
