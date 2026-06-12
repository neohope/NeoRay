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
	Database DatabaseConfig `mapstructure:"database"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Session  SessionConfig  `mapstructure:"session"`
	Tools    ToolsConfig    `mapstructure:"tools"`
	Channels ChannelsConfig `mapstructure:"channels"`
	Security SecurityConfig `mapstructure:"security"`
	Web      WebConfig      `mapstructure:"web"`

	// 内部字段
	HomeDir string `mapstructure:"-"` // 用户数据目录 ~/.neoray
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"`
}

// ServerConfig 服务端配置
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	CORS         CORSConfig    `mapstructure:"cors"`
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
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
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver   string            `mapstructure:"driver"`
	SQLite   SQLiteConfig      `mapstructure:"sqlite"`
	Postgres PostgresConfig    `mapstructure:"postgres"`
}

// SQLiteConfig SQLite 配置
type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

// PostgresConfig PostgreSQL 配置
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// LLMConfig LLM 配置
type LLMConfig struct {
	DefaultProvider string           `mapstructure:"default_provider"`
	Anthropic       AnthropicConfig  `mapstructure:"anthropic"`
	OpenAI          OpenAIConfig     `mapstructure:"openai"`
}

// AnthropicConfig Anthropic 配置
type AnthropicConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	APIURL      string        `mapstructure:"api_url"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float64       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

// OpenAIConfig OpenAI 配置
type OpenAIConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	APIURL      string        `mapstructure:"api_url"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float64       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

// SessionConfig 会话配置
type SessionConfig struct {
	Storage SessionStorageConfig `mapstructure:"storage"`
	Context SessionContextConfig `mapstructure:"context"`
}

// SessionStorageConfig 会话存储配置
type SessionStorageConfig struct {
	Type                 string `mapstructure:"type"`
	MaxSessions          int    `mapstructure:"max_sessions"`
	MaxMessagesPerSession int   `mapstructure:"max_messages_per_session"`
}

// SessionContextConfig 会话上下文配置
type SessionContextConfig struct {
	MaxTokens            int     `mapstructure:"max_tokens"`
	CompressionStrategy  string  `mapstructure:"compression_strategy"`
	AutoSummarize        bool    `mapstructure:"auto_summarize"`
	SummarizeThreshold   float64 `mapstructure:"summarize_threshold"`
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	Workspace  WorkspaceConfig  `mapstructure:"workspace"`
	Filesystem FilesystemConfig `mapstructure:"filesystem"`
	Shell      ShellConfig      `mapstructure:"shell"`
	Web        WebToolsConfig   `mapstructure:"web"`
	Cron       CronConfig       `mapstructure:"cron"`
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
	Enabled        bool          `mapstructure:"enabled"`
	Timeout        time.Duration `mapstructure:"timeout"`
	AllowedCommands []string    `mapstructure:"allowed_commands"`
	BlockedCommands []string    `mapstructure:"blocked_commands"`
	WorkingDir     string        `mapstructure:"working_dir"`
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
type ChannelsConfig struct {
	WebSocket WebSocketChannelConfig `mapstructure:"websocket"`
	Feishu    FeishuChannelConfig    `mapstructure:"feishu"`
}

// WebSocketChannelConfig WebSocket 频道配置
type WebSocketChannelConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Path            string        `mapstructure:"path"`
	PingInterval    time.Duration `mapstructure:"ping_interval"`
	WriteWait       time.Duration `mapstructure:"write_wait"`
	PongWait        time.Duration `mapstructure:"pong_wait"`
	MaxMessageSize  int64         `mapstructure:"max_message_size"`
}

// FeishuChannelConfig 飞书频道配置
type FeishuChannelConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	AppID           string `mapstructure:"app_id"`
	AppSecret       string `mapstructure:"app_secret"`
	VerificationToken string `mapstructure:"verification_token"`
	EncryptKey      string `mapstructure:"encrypt_key"`
	WebhookPath     string `mapstructure:"webhook_path"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	Auth      AuthConfig      `mapstructure:"auth"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Upload    UploadConfig    `mapstructure:"upload"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	SecretKey  string        `mapstructure:"secret_key"`
	JWTExpiry  time.Duration `mapstructure:"jwt_expiry"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool  `mapstructure:"enabled"`
	RequestsPerMinute int   `mapstructure:"requests_per_minute"`
	Burst             int   `mapstructure:"burst"`
}

// UploadConfig 上传配置
type UploadConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	MaxSize      int64    `mapstructure:"max_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
	TempDir      string   `mapstructure:"temp_dir"`
}

// WebConfig Web UI 配置
type WebConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	StaticDir string `mapstructure:"static_dir"`
	IndexPath string `mapstructure:"index_path"`
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
func GetWorkspace() string {
	homeDir := GetHomeDir()
	// 默认工作区是当前目录
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return filepath.Join(homeDir, "workspace")
}
