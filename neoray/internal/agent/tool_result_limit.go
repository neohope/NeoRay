package agent

import (
	"neoray/internal/config"
	"neoray/internal/tools"
)

// DefaultMaxToolResultChars 工具结果进入上下文前的默认字符上限。
const DefaultMaxToolResultChars = tools.DefaultMaxToolResultChars

// effectiveMaxToolResultChars 返回工具结果进入上下文的字符上限。
// 配置为 0 时使用默认值（16000），负数视为不限制（仅供测试使用）。
func effectiveMaxToolResultChars(cfg *config.Config) int {
	if cfg == nil {
		return DefaultMaxToolResultChars
	}
	if cfg.Agent.MaxToolResultChars > 0 {
		return cfg.Agent.MaxToolResultChars
	}
	if cfg.Agent.MaxToolResultChars < 0 {
		return 0
	}
	return DefaultMaxToolResultChars
}
