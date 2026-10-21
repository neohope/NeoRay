package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// removedConfigPrefixes 记录「已经从结构体移除」的配置前缀。
//
// 这些配置曾经存在于示例文件中，但对应功能从未实现（或已被删除）。
// 用户的老配置文件里很可能还留着它们，读取时会被 mapstructure 静默丢弃，
// 所以这里主动检测并告警，避免"我明明配了"的困惑。
var removedConfigPrefixes = map[string]string{
	"database":                     "当前版本不使用数据库（会话与记忆均为文件存储）",
	"channels.websocket":           "当前版本没有 WebSocket 频道实现（Web UI 的浏览器 WS 属于 web 配置）",
	"security.upload":              "当前版本没有上传接口",
	"session.context":              "上下文压缩策略由 agent 内部固定实现，不接受配置",
	"server.cors":                  "只有 allowed_origins 生效，其余 CORS 子项已移除",
	"agent.tool_hint_max_length":   "未实现",
	"agent.context_block_limit":    "未实现",
	"agent.max_messages":           "未实现；请改用 session.storage.max_messages_per_session",
	"agent.consolidation_ratio":    "未实现",
	"agent.session_ttl_minutes":    "与 memory.session_ttl_minutes 重复，请使用后者",
	"agent.unified_session":        "未实现",
	"tools.shell.allowed_commands": "只实现了 blocked_commands，白名单已移除",
	"web.index_path":               "未实现",
}

// detectDeprecatedKeys 找出配置文件里仍然存在的已移除配置键（去重后按前缀归并）
func detectDeprecatedKeys(v *viper.Viper) []string {
	if v == nil {
		return nil
	}
	found := make(map[string]bool)
	for _, key := range v.AllKeys() {
		// 只看配置文件里真正写了的键；否则 SetDefault 会让我们误报
		if !v.InConfig(key) {
			continue
		}
		lower := strings.ToLower(key)
		for prefix := range removedConfigPrefixes {
			if lower == prefix || strings.HasPrefix(lower, prefix+".") {
				found[prefix] = true
			}
		}
	}
	if len(found) == 0 {
		return nil
	}
	out := make([]string, 0, len(found))
	for prefix := range found {
		out = append(out, prefix)
	}
	sort.Strings(out)
	return out
}

// UnusedConfigWarnings 返回「配置了但当前版本不生效」的说明。
func UnusedConfigWarnings(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	warns := make([]string, 0, len(cfg.DeprecatedKeys))
	for _, key := range cfg.DeprecatedKeys {
		reason := removedConfigPrefixes[key]
		if reason == "" {
			reason = "当前版本不生效"
		}
		warns = append(warns, fmt.Sprintf("[config] %s 已移除或不生效：%s", key, reason))
	}
	return warns
}
