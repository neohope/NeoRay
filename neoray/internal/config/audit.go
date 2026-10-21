package config

import "fmt"

// 本文件解决一类反复出现的问题：**配置项存在，但代码从不读取**。
//
// 典型来源是「先设计配置、后写实现」，实现没跟上时配置就变成了谎言：
// 用户设了 blocked_domains / max_jobs / database.*，行为却毫无变化，
// 排查时还以为是 bug（飞书机器人的"我查不到配置"就是这么来的）。
//
// 已接入的项不在这里；这里只列**当前版本确实不生效**的配置，
// 启动时打 warning，把"沉默地忽略"变成"明确告诉你"。

// UnusedConfigWarnings 返回「配置了但当前版本不生效」的项
func UnusedConfigWarnings(cfg *Config) []string {
	if cfg == nil {
		return nil
	}

	var warns []string
	add := func(cond bool, format string, args ...any) {
		if cond {
			warns = append(warns, fmt.Sprintf(format, args...))
		}
	}

	// 数据库：当前版本会话/记忆走文件存储，没有任何 DB 代码
	add(cfg.Database.Driver != "" || cfg.Database.SQLite.Path != "" ||
		cfg.Database.Postgres.Host != "",
		"[config] [database] 已配置但当前版本不使用数据库（会话与记忆均为文件存储），这些配置不会生效")

	// WebSocket 通道：仅有 Web UI 的浏览器 WS，channels.websocket 无实现
	ws := cfg.Channels.WebSocket
	add(ws.Path != "" || ws.PingInterval > 0 || ws.PongWait > 0 ||
		ws.WriteWait > 0 || ws.MaxMessageSize > 0,
		"[config] [channels.websocket] 已配置但当前版本没有 WebSocket 通道实现，这些配置不会生效")

	// 上传：当前版本没有上传接口
	up := cfg.Security.Upload
	add(up.Enabled || up.MaxSize > 0 || len(up.AllowedTypes) > 0 || up.TempDir != "",
		"[config] [security.upload] 已配置但当前版本没有上传接口，这些配置不会生效")

	// CORS 的细粒度开关：实现只读取 allowed_origins
	add(len(cfg.Server.CORS.AllowedMethods) > 0 || len(cfg.Server.CORS.AllowedHeaders) > 0,
		"[config] [server.cors] 只有 allowed_origins 生效；allowed_methods/allowed_headers 不会生效")

	// 上下文压缩与会话上下文参数
	sc := cfg.Session.Context
	add(sc.MaxTokens > 0 || sc.CompressionStrategy != "" || sc.AutoSummarize || sc.SummarizeThreshold > 0,
		"[config] [session.context] 已配置但压缩策略由 agent 内部固定实现，这些配置不会生效")

	// Agent 侧未接线项
	add(cfg.Agent.ToolHintMaxLength > 0,
		"[config] agent.tool_hint_max_length 未接线，不会生效")
	add(cfg.Agent.ContextBlockLimit > 0,
		"[config] agent.context_block_limit 未接线，不会生效")
	add(cfg.Agent.MaxMessages > 0,
		"[config] agent.max_messages 未接线；历史上限见 session.storage.max_messages_per_session")
	add(cfg.Agent.ConsolidationRatio > 0,
		"[config] agent.consolidation_ratio 未接线，不会生效")
	add(cfg.Agent.SessionTTLMinutes > 0,
		"[config] agent.session_ttl_minutes 未接线；实际生效的是 memory.session_ttl_minutes")

	// Shell 白名单：只实现了 blocked_commands
	add(len(cfg.Tools.Shell.AllowedCommands) > 0,
		"[config] tools.shell.allowed_commands 未接线（只实现了 blocked_commands），不会生效")

	// Web UI 索引路径
	add(cfg.Web.IndexPath != "",
		"[config] web.index_path 未接线，不会生效")

	return warns
}
