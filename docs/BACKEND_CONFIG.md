
# 后端配置设计

## 配置文件格式

采用 **TOML** 作为配置文件格式，因为它：
- 易于阅读和编辑
- 支持注释
- 支持嵌套结构
- Go 生态成熟 (`github.com/spf13/viper`)
- 比 YAML 更简单明确，没有缩进问题

---

## 目录结构

### 用户目录结构 (运行时)
配置、日志、数据默认存储在当前用户的 `.neoray` 文件夹中：

```
# Windows
C:\Users\YourName\.neoray\
├── config.toml           # 主配置文件
├── config.local.toml     # 本地覆盖 (可选)
├── logs/                 # 日志目录
│   └── neoray.log
├── data/                 # 数据目录
│   └── neoray.db         # SQLite 数据库
└── workspace/            # 默认工作区
    └── (工具可操作的文件)

# macOS / Linux
~/
└── .neoray/
    ├── config.toml
    ├── logs/
    ├── data/
    └── workspace/
```

### 项目目录结构 (开发时)
```
NeoRay/
├── neoray/               # Go 后端 (项目名: neoray)
│   ├── cmd/
│   ├── internal/
│   └── pkg/
├── neorayui/             # Flutter 前端 (项目名: neorayui)
├── config/               # 默认配置模板 (开发时)
│   ├── config.toml       # 默认配置模板
│   └── config.dev.toml   # 开发配置模板
└── docs/                 # 文档
```

---

## 配置文件位置优先级 (从高到低)

1. **命令行参数**: `--config /path/to/config.toml`
2. **环境变量**: `NEORAY_CONFIG=/path/to/config.toml`
3. **用户目录**: `~/.neoray/config.toml`
4. **当前目录**: `./config.toml`
5. **可执行文件同目录**: `./config.toml`

---

## 配置文件结构 (config.toml)

```toml
# 应用基本信息
[app]
name = "neoray"
version = "0.1.0"
env = "development"  # development, staging, production

# 服务器配置
[server]
host = "127.0.0.1"  # 默认仅监听本地
port = 8080
read_timeout = "60s"
write_timeout = "60s"
idle_timeout = "120s"

# CORS 配置
[server.cors]
enabled = true
allowed_origins = [
  "http://localhost:*",
  "http://127.0.0.1:*"
]
allowed_methods = [
  "GET",
  "POST",
  "PUT",
  "DELETE",
  "OPTIONS"
]
allowed_headers = ["*"]
allow_credentials = true

# 日志配置
[logger]
level = "info"  # debug, info, warn, error, fatal
format = "json"  # json, text
output = ["stdout", "file"]

[logger.file]
# 相对路径: 相对于 ~/.neoray/
# 绝对路径: /var/log/neoray/
path = "logs/neoray.log"
max_size = 100  # MB
max_backups = 3
max_age = 28  # days
compress = true

# 数据库配置
[database]
driver = "sqlite"  # sqlite, postgres

# 开发环境用 SQLite
[database.sqlite]
# 相对路径: 相对于 ~/.neoray/
path = "data/neoray.db"

# 生产环境用 PostgreSQL
[database.postgres]
host = "localhost"
port = 5432
user = "neoray"
password = ""
dbname = "neoray"
sslmode = "disable"

# LLM 提供商配置
[llm]
default_provider = "anthropic"

# Anthropic Claude
[llm.anthropic]
api_key = ""
api_url = "https://api.anthropic.com"
model = "claude-3-sonnet-20240229"
max_tokens = 4096
temperature = 0.7
timeout = "120s"

# OpenAI 兼容 (可用于本地模型)
[llm.openai]
api_key = ""
api_url = "https://api.openai.com/v1"
model = "gpt-4"
max_tokens = 4096
temperature = 0.7
timeout = "120s"

# 会话配置
[session]

# 会话存储
[session.storage]
type = "sqlite"  # sqlite, postgres, memory
max_sessions = 1000
max_messages_per_session = 10000

# 上下文管理
[session.context]
max_tokens = 128000
compression_strategy = "truncate"  # truncate, summarize, hybrid
auto_summarize = true
summarize_threshold = 0.8  # 当达到 max_tokens 的 80% 时开始压缩

# 工具配置
[tools]

# 工作区限制
[tools.workspace]
enabled = true
# 相对路径: 相对于 ~/.neoray/
allowed_paths = ["workspace"]
read_only = false

# 文件系统工具
[tools.filesystem]
enabled = true
max_file_size = 10485760  # 10MB
allowed_extensions = [".*"]  # 所有文件
blocked_extensions = [".exe", ".dll", ".so", ".dylib"]

# Shell 工具
[tools.shell]
enabled = true
timeout = "30s"
allowed_commands = [
  "ls",
  "cat",
  "grep",
  "git",
  "python",
  "node"
]
blocked_commands = [
  "rm -rf",
  "mkfs",
  ":(){ :|:&amp; };:"
]
# 相对路径: 相对于 ~/.neoray/
working_dir = "workspace"

# Web 工具
[tools.web]
enabled = true
timeout = "30s"
max_content_length = 10485760  # 10MB
blocked_domains = ["internal.example.com"]

# 定时任务
[tools.cron]
enabled = true
max_jobs = 100

# 频道配置
[channels]

# WebSocket 频道
[channels.websocket]
enabled = true
path = "/ws"
ping_interval = "30s"
write_wait = "10s"
pong_wait = "60s"
max_message_size = 4194304  # 4MB

# 飞书频道
[channels.feishu]
enabled = false
app_id = ""
app_secret = ""
verification_token = ""
encrypt_key = ""
webhook_path = "/webhook/feishu"

# 安全配置
[security]

# 认证
[security.auth]
enabled = false
secret_key = "your-secret-key-change-in-production"
jwt_expiry = "24h"

# 请求限制
[security.rate_limit]
enabled = true
requests_per_minute = 60
burst = 100

# 文件上传
[security.upload]
enabled = true
max_size = 10485760  # 10MB
allowed_types = [
  "text/*",
  "image/*",
  "application/json"
]
# 相对路径: 相对于 ~/.neoray/
temp_dir = "tmp/uploads"

# Web UI 配置 (可选，用于托管前端)
[web]
enabled = false
static_dir = "../neorayui/dist"
index_path = "../neorayui/dist/index.html"
```

---

## 环境变量映射

环境变量格式：`NEORAY_` 前缀 + 路径（下划线分隔）+ 大写

例如：

| TOML 路径 | 环境变量 |
|----------|---------|
| `server.port` | `NEORAY_SERVER_PORT` |
| `llm.anthropic.api_key` | `NEORAY_LLM_ANTHROPIC_API_KEY` |
| `database.postgres.password` | `NEORAY_DATABASE_POSTGRES_PASSWORD` |

特殊环境变量：
- `NEORAY_CONFIG`: 配置文件路径
- `NEORAY_HOME`: 用户数据目录 (默认 `~/.neoray`)

---

## Go 配置结构 (internal/config/config.go)

```go
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
	Level  string         `mapstructure:"level"`
	Format string         `mapstructure:"format"`
	Output []string       `mapstructure:"output"`
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
	Enabled        bool     `mapstructure:"enabled"`
	AllowedPaths   []string `mapstructure:"allowed_paths"`
	ReadOnly       bool     `mapstructure:"read_only"`
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
```

---

## 配置加载实现 (internal/config/loader.go)

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Load 加载配置
func Load(configPath string) (*Config, error) {
	homeDir := GetHomeDir()

	// 确保用户目录存在
	if err := ensureHomeDir(homeDir); err != nil {
		return nil, fmt.Errorf("create home dir: %w", err)
	}

	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 配置文件类型
	v.SetConfigType("toml")

	// 1. 从命令行参数指定的路径加载
	// 2. 从环境变量 NEORAY_CONFIG 指定的路径加载
	// 3. 从 ~/.neoray/config.toml 加载
	// 4. 从 ./config.toml 加载
	// 5. 从可执行文件同目录的 config.toml 加载

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 添加搜索路径
		v.AddConfigPath(homeDir)
		v.AddConfigPath(".")

		// 可执行文件同目录
		if execPath, err := os.Executable(); err == nil {
			v.AddConfigPath(filepath.Dir(execPath))
		}

		v.SetConfigName("config")
	}

	// 尝试加载配置
	if err := v.ReadInConfig(); err != nil {
		// 配置文件不存在，使用默认配置
		fmt.Printf("Config file not found, using defaults. Creating in %s\n", homeDir)

		// 保存默认配置到用户目录
		cfg := &Config{HomeDir: homeDir}
		if err := v.Unmarshal(cfg); err != nil {
			return nil, fmt.Errorf("unmarshal default config: %w", err)
		}

		// 保存默认配置文件
		if err := saveDefaultConfig(filepath.Join(homeDir, "config.toml")); err != nil {
			fmt.Printf("Warning: failed to save default config: %v\n", err)
		}

		return cfg, nil
	}

	// 加载本地覆盖配置 (可选)
	v.SetConfigName("config.local")
	_ = v.MergeInConfig()

	// 环境变量覆盖
	v.SetEnvPrefix("NEORAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 解析到结构体
	var cfg Config
	if err := v.Unmarshal(&amp;cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.HomeDir = homeDir

	// 验证配置
	if err := validate(&amp;cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// 确保必需的子目录存在
	if err := ensureSubDirs(&amp;cfg); err != nil {
		return nil, fmt.Errorf("create sub dirs: %w", err)
	}

	return &amp;cfg, nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	// App
	v.SetDefault("app.name", "neoray")
	v.SetDefault("app.version", "0.1.0")
	v.SetDefault("app.env", "development")

	// Server
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "60s")
	v.SetDefault("server.write_timeout", "60s")
	v.SetDefault("server.idle_timeout", "120s")

	// Logger
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "json")
	v.SetDefault("logger.output", []string{"stdout", "file"})
	v.SetDefault("logger.file.path", "logs/neoray.log")
	v.SetDefault("logger.file.max_size", 100)
	v.SetDefault("logger.file.max_backups", 3)
	v.SetDefault("logger.file.max_age", 28)
	v.SetDefault("logger.file.compress", true)

	// Database
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.sqlite.path", "data/neoray.db")

	// LLM
	v.SetDefault("llm.default_provider", "anthropic")
	v.SetDefault("llm.anthropic.api_url", "https://api.anthropic.com")
	v.SetDefault("llm.anthropic.model", "claude-3-sonnet-20240229")
	v.SetDefault("llm.anthropic.max_tokens", 4096)
	v.SetDefault("llm.anthropic.temperature", 0.7)
	v.SetDefault("llm.anthropic.timeout", "120s")

	// Session
	v.SetDefault("session.storage.type", "sqlite")
	v.SetDefault("session.storage.max_sessions", 1000)
	v.SetDefault("session.storage.max_messages_per_session", 10000)
	v.SetDefault("session.context.max_tokens", 128000)
	v.SetDefault("session.context.compression_strategy", "truncate")
	v.SetDefault("session.context.auto_summarize", true)
	v.SetDefault("session.context.summarize_threshold", 0.8)

	// Tools
	v.SetDefault("tools.workspace.enabled", true)
	v.SetDefault("tools.workspace.allowed_paths", []string{"workspace"})
	v.SetDefault("tools.filesystem.enabled", true)
	v.SetDefault("tools.filesystem.max_file_size", 10485760)
	v.SetDefault("tools.shell.enabled", true)
	v.SetDefault("tools.shell.timeout", "30s")
	v.SetDefault("tools.shell.working_dir", "workspace")
	v.SetDefault("tools.web.enabled", true)
	v.SetDefault("tools.web.timeout", "30s")
	v.SetDefault("tools.cron.enabled", true)
	v.SetDefault("tools.cron.max_jobs", 100)

	// Channels
	v.SetDefault("channels.websocket.enabled", true)
	v.SetDefault("channels.websocket.path", "/ws")
	v.SetDefault("channels.websocket.ping_interval", "30s")
	v.SetDefault("channels.websocket.write_wait", "10s")
	v.SetDefault("channels.websocket.pong_wait", "60s")
	v.SetDefault("channels.websocket.max_message_size", 4194304)
	v.SetDefault("channels.feishu.enabled", false)

	// Security
	v.SetDefault("security.auth.enabled", false)
	v.SetDefault("security.rate_limit.enabled", true)
	v.SetDefault("security.rate_limit.requests_per_minute", 60)
	v.SetDefault("security.rate_limit.burst", 100)
	v.SetDefault("security.upload.enabled", true)
	v.SetDefault("security.upload.max_size", 10485760)
	v.SetDefault("security.upload.temp_dir", "tmp/uploads")

	// Web
	v.SetDefault("web.enabled", false)
}

// validate 验证配置
func validate(cfg *Config) error {
	if cfg.App.Env == "production" {
		if cfg.LLM.Anthropic.APIKey == "" &amp;&amp; cfg.LLM.OpenAI.APIKey == "" {
			return fmt.Errorf("llm api key is required in production")
		}
		if cfg.Database.Driver == "sqlite" {
			// 生产环境建议用 PostgreSQL
		}
	}
	return nil
}

// ensureHomeDir 确保用户目录存在
func ensureHomeDir(homeDir string) error {
	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		return os.MkdirAll(homeDir, 0755)
	}
	return nil
}

// ensureSubDirs 确保子目录存在
func ensureSubDirs(cfg *Config) error {
	dirs := []string{
		cfg.ResolvePath("logs"),
		cfg.ResolvePath("data"),
		cfg.ResolvePath("workspace"),
		cfg.ResolvePath("tmp/uploads"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

// saveDefaultConfig 保存默认配置文件
func saveDefaultConfig(path string) error {
	// 检查是否已存在
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	// 创建默认配置内容
	defaultConfig := `# NeoRay 配置文件
# 配置文件自动生成于: ~/.neoray/config.toml

[app]
name = "neoray"
version = "0.1.0"
env = "development"

[server]
host = "127.0.0.1"
port = 8080

[logger]
level = "info"
output = ["stdout", "file"]

[database]
driver = "sqlite"

[llm]
default_provider = "anthropic"
`

	return os.WriteFile(path, []byte(defaultConfig), 0644)
}

// GetEnv 获取当前环境
func GetEnv() string {
	env := os.Getenv("NEORAY_ENV")
	if env == "" {
		env = "development"
	}
	return env
}
```

---

## 使用示例

```go
package main

import (
	"log"

	"neoray/internal/config"
)

func main() {
	// 加载配置 (自动在 ~/.neoray/ 中查找)
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 使用配置
	log.Printf("Starting %s v%s", cfg.App.Name, cfg.App.Version)
	log.Printf("Home dir: %s", cfg.HomeDir)
	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("LLM Provider: %s", cfg.LLM.DefaultProvider)

	// 解析相对路径为绝对路径
	dbPath := cfg.ResolvePath(cfg.Database.SQLite.Path)
	log.Printf("Database path: %s", dbPath)
}
```

---

## .gitignore

```
# 用户数据
.neoray/

# 配置 (本地覆盖)
config/config.local.toml
config/config.prod.toml

# IDE
.idea/
.vscode/

# 构建
neoray/server
*.exe

# 其他
*.db
*.log
```

