package config

import (
	"os"
	"path/filepath"
	"time"
)

// Config 全局配置
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Logger   LoggerConfig   `mapstructure:"logger"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Memory   MemoryConfig   `mapstructure:"memory"`
	Session  SessionConfig  `mapstructure:"session"`
	Tools    ToolsConfig    `mapstructure:"tools"`
	Skills   SkillsConfig   `mapstructure:"skills"`
	Channels ChannelsConfig `mapstructure:"channels"`
	Security SecurityConfig `mapstructure:"security"`
	Web      WebConfig      `mapstructure:"web"`
	Agent    AgentConfig    `mapstructure:"agent"`

	// 内部字段
	HomeDir    string `mapstructure:"-"` // 用户数据目录 ~/.neoray
	ConfigPath string `mapstructure:"-"` // 实际加载的配置文件路径（供写回使用）
	// DeprecatedKeys 记录配置文件里仍然存在、但当前版本已移除/不生效的配置键，
	// 启动时由 UnusedConfigWarnings() 输出，避免"设了没用"的静默忽略
	DeprecatedKeys []string `mapstructure:"-"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"`
}

// CORSConfig 跨域配置
//
// 只有 allowed_origins 与 allow_credentials 生效；
// allowed_methods / allowed_headers 从未接线，已移除。
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	// WSMessageTimeout 单条 WebSocket 消息的处理超时。
	// 必须大于 agent 自身的处理超时（内部为 5 分钟），否则长任务
	// （多轮工具调用）会被外层先截断，导致 chat_end 无法下发。
	WSMessageTimeout time.Duration `mapstructure:"ws_message_timeout"`
	// CORS 跨域配置
	CORS CORSConfig `mapstructure:"cors"`
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level  string           `mapstructure:"level"`
	Format string           `mapstructure:"format"`
	Output []string         `mapstructure:"output"`
	File   LoggerFileConfig `mapstructure:"file"`
}

// LoggerFileConfig 日志文件配置
type LoggerFileConfig struct {
	Path        string `mapstructure:"path"`
	MaxSize     int    `mapstructure:"max_size"`
	MaxBackups  int    `mapstructure:"max_backups"`
	MaxAge      int    `mapstructure:"max_age"`
	Compress    bool   `mapstructure:"compress"`
	RotateDaily bool   `mapstructure:"rotate_daily"`
}

// LLMConfig LLM 配置
type LLMConfig struct {
	DefaultProvider string                    `mapstructure:"default_provider"`
	Providers       map[string]ProviderConfig `mapstructure:",remain"`
	FallbackModels  []FallbackModelConfig     `mapstructure:"fallback_models"`
}

// FallbackModelConfig Fallback 模型配置
type FallbackModelConfig struct {
	Model           string  `mapstructure:"model"`
	Provider        string  `mapstructure:"provider"`
	MaxTokens       int     `mapstructure:"max_tokens,omitempty"`
	Temperature     float64 `mapstructure:"temperature,omitempty"`
	ReasoningEffort string  `mapstructure:"reasoning_effort,omitempty"`
}

// ProviderConfig 通用提供商配置
type ProviderConfig struct {
	APIKey             string        `mapstructure:"api_key"`
	APIURL             string        `mapstructure:"api_url"`
	Model              string        `mapstructure:"model"`
	MaxTokens          int           `mapstructure:"max_tokens"`
	Temperature        float64       `mapstructure:"temperature"`
	Timeout            time.Duration `mapstructure:"timeout"`
	APIFormat          string        `mapstructure:"api_format"`                 // "openai" 或 "anthropic"，默认为 "openai"
	ReasoningEffort    string        `mapstructure:"reasoning_effort,omitempty"` // "low", "medium", "high", "adaptive", "none"
	PromptCacheEnabled bool          `mapstructure:"prompt_cache_enabled,omitempty"`
}

// MemoryConfig 记忆系统配置
type MemoryConfig struct {
	// 工作区目录（存放 SOUL.md, USER.md, memory/）
	Workspace string `mapstructure:"workspace"`
	// 是否启用 Git 版本控制
	GitEnabled bool `mapstructure:"git_enabled"`
	// Dream 处理间隔
	DreamInterval string `mapstructure:"dream_interval"`
	// 会话 TTL（分钟，0 表示不自动归档）
	SessionTTLMinutes int `mapstructure:"session_ttl_minutes"`
	// 最大历史条目数
	MaxHistoryEntries int `mapstructure:"max_history_entries"`
}

// SessionConfig 会话配置
type SessionConfig struct {
	Storage SessionStorageConfig `mapstructure:"storage"`
	Context SessionContextConfig `mapstructure:"context"`
}

// SessionContextConfig 上下文配置
//
// 只有 max_tokens 生效（用于历史截断）；历史上的
// compression_strategy / auto_summarize / summarize_threshold 从未接线，已移除。
type SessionContextConfig struct {
	MaxTokens int `mapstructure:"max_tokens"`
}

// SessionStorageConfig 会话存储配置
type SessionStorageConfig struct {
	Type                  string `mapstructure:"type"`
	MaxSessions           int    `mapstructure:"max_sessions"`
	MaxMessagesPerSession int    `mapstructure:"max_messages_per_session"`
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	Workspace     WorkspaceConfig   `mapstructure:"workspace"`
	Filesystem    FilesystemConfig  `mapstructure:"filesystem"`
	Shell         ShellConfig       `mapstructure:"shell"`
	Web           WebToolsConfig    `mapstructure:"web"`
	Cron          CronConfig        `mapstructure:"cron"`
	Subagent      SubagentConfig    `mapstructure:"subagent"`
	FindFiles     GenericToolConfig `mapstructure:"find_files"`
	Grep          GenericToolConfig `mapstructure:"grep"`
	ApplyPatch    GenericToolConfig `mapstructure:"apply_patch"`
	WebSearch     GenericToolConfig `mapstructure:"web_search"`
	WebFetch      GenericToolConfig `mapstructure:"web_fetch"`
	SandboxStatus GenericToolConfig `mapstructure:"sandbox_status"`
}

// GenericToolConfig 通用工具配置
type GenericToolConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// SubagentConfig 子代理配置
type SubagentConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	MaxConcurrent      int  `mapstructure:"max_concurrent"`
	MaxIterations      int  `mapstructure:"max_iterations"`
	MaxToolResultChars int  `mapstructure:"max_tool_result_chars"`
}

// WorkspaceConfig 工作区配置
type WorkspaceConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	AllowedPaths []string `mapstructure:"allowed_paths"`
	ReadOnly     bool     `mapstructure:"read_only"`
}

// FilesystemConfig 文件系统工具配置
type FilesystemConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	MaxFileSize       int64    `mapstructure:"max_file_size"`
	AllowedExtensions []string `mapstructure:"allowed_extensions"`
	BlockedExtensions []string `mapstructure:"blocked_extensions"`
}

// ShellConfig Shell 工具配置
type ShellConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Timeout         time.Duration `mapstructure:"timeout"`
	BlockedCommands []string      `mapstructure:"blocked_commands"`
	WorkingDir      string        `mapstructure:"working_dir"`
	Sandbox         string        `mapstructure:"sandbox"`
	MediaDir        string        `mapstructure:"media_dir"`
}

// WebToolsConfig Web 工具配置
type WebToolsConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	Timeout          time.Duration `mapstructure:"timeout"`
	MaxContentLength int64         `mapstructure:"max_content_length"`
	BlockedDomains   []string      `mapstructure:"blocked_domains"`
}

// CronConfig 定时任务配置
type CronConfig struct {
	Enabled bool `mapstructure:"enabled"`
	MaxJobs int  `mapstructure:"max_jobs"`
}

// ChannelsConfig 频道配置
//
// 只保留有实现的两个频道；历史上的 channels.websocket 从未落地
// （Web UI 的浏览器 WS 属于 web 配置，与此无关），已从结构体中移除。
type ChannelsConfig struct {
	Feishu FeishuChannelConfig `mapstructure:"feishu"`
}

// FeishuChannelConfig 飞书频道配置
type FeishuChannelConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	AppID             string `mapstructure:"app_id"`
	AppSecret         string `mapstructure:"app_secret"`
	VerificationToken string `mapstructure:"verification_token"`
	EncryptKey        string `mapstructure:"encrypt_key"`
	Domain            string `mapstructure:"domain"`           // "feishu" 或 "lark"
	GroupPolicy       string `mapstructure:"group_policy"`     // "mention" 或 "open"
	ReplyToMessage    bool   `mapstructure:"reply_to_message"` // 是否引用原消息回复
	TopicIsolation    bool   `mapstructure:"topic_isolation"`  // 是否话题隔离
	ReactEmoji        string `mapstructure:"react_emoji"`      // 处理中的表情，默认 "THINKING"
	DoneEmoji         string `mapstructure:"done_emoji"`       // 完成时的表情
	ToolHintPrefix    string `mapstructure:"tool_hint_prefix"` // 工具提示前缀，默认 "🔧"
	Streaming         bool   `mapstructure:"streaming"`        // 是否启用流式响应
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	Auth                         AuthConfig      `mapstructure:"auth"`
	RateLimit                    RateLimitConfig `mapstructure:"rate_limit"`
	RestrictToWorkspace          bool            `mapstructure:"restrict_to_workspace"`
	WebUIAllowLocalServiceAccess bool            `mapstructure:"webui_allow_local_service_access"`
	SSRFWhitelist                []string        `mapstructure:"ssrf_whitelist"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	SecretKey  string        `mapstructure:"secret_key"`
	AdminToken string        `mapstructure:"admin_token"`
	JWTExpiry  time.Duration `mapstructure:"jwt_expiry"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerMinute int  `mapstructure:"requests_per_minute"`
	Burst             int  `mapstructure:"burst"`
}

// SkillsConfig Skills 配置
type SkillsConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	BuiltinSkillsDir string   `mapstructure:"builtin_skills_dir"`
	DisabledSkills   []string `mapstructure:"disabled_skills"`
	AutoLoadAlways   bool     `mapstructure:"auto_load_always"` // 自动加载标记为 always=true 的 skills
}

// AgentConfig Agent 配置
type AgentConfig struct {
	// 最大工具调用迭代次数
	MaxIterations int `mapstructure:"max_iterations"`
	// 是否使用统一会话（所有来源共用一个会话）
	UnifiedSession bool `mapstructure:"unified_session"`
	// 上下文窗口大小
	ContextWindowTokens int `mapstructure:"context_window_tokens"`
	// 最大工具结果字符数
	MaxToolResultChars int `mapstructure:"max_tool_result_chars"`
	// 提供者重试模式
	ProviderRetryMode string `mapstructure:"provider_retry_mode"`
	// 单次工具调用的外层超时；0 表示按内层工具超时自动推算
	ToolTimeout time.Duration `mapstructure:"tool_timeout"`
	// 单条消息整轮处理超时（含多轮工具调用）；0 表示默认 10 分钟
	MessageTimeout time.Duration `mapstructure:"message_timeout"`
	// 时区
	Timezone string `mapstructure:"timezone"`
	// 禁用的技能
	DisabledSkills []string `mapstructure:"disabled_skills"`
}

// WebConfig Web UI 配置
type WebConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	StaticDir string `mapstructure:"static_dir"`
	IndexPath string `mapstructure:"index_path"`
	// DevOrigin 为开发模式下前端 dev server 的 Origin（如 http://127.0.0.1:5173），
	// 非空时会加入 CORS 白名单，仅用于本地调试。
	DevOrigin string `mapstructure:"dev_origin"`
}

// GetHomeDir 获取用户数据目录 ~/.neoray
func GetHomeDir() string {
	// 优先从环境变量获取
	if home := os.Getenv("NEORAY_HOME"); home != "" {
		return home
	}

	// 获取用户主目录
	userHome, err := os.UserHomeDir()
	if err != nil {
		// 失败时使用当前目录
		return "./.neoray"
	}

	return filepath.Join(userHome, ".neoray")
}

// ResolvePath 解析相对路径为绝对路径
func (c *Config) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.HomeDir, path)
}

// GetWorkspace 获取工作区目录 (全局函数，用于不需要 Config 实例的场景)
// 默认为 ~/.neoray/workspace
func GetWorkspace() string {
	homeDir := GetHomeDir()
	return filepath.Join(homeDir, "workspace")
}
