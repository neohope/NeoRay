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
	// 3. 从 ~/.neoray/neoray.toml 加载
	// 4. 从 ./neoray.toml 加载
	// 5. 从可执行文件同目录的 neoray.toml 加载

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 添加搜索路径
		v.AddConfigPath(homeDir)
		v.AddConfigPath(".")
		v.AddConfigPath("../config")

		// 可执行文件同目录
		if execPath, err := os.Executable(); err == nil {
			v.AddConfigPath(filepath.Dir(execPath))
		}

		v.SetConfigName("config")
	}

	// 尝试加载配置
	if err := v.ReadInConfig(); err != nil {
		fmt.Printf("Config file not found, using defaults\n")
	} else {
		fmt.Printf("✅ Loaded config from: %s\n", v.ConfigFileUsed())
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
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.HomeDir = homeDir

	// 如果用户目录没有配置文件，保存默认配置
	userConfigPath := filepath.Join(homeDir, "neoray.toml")
	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		fmt.Printf("Creating default config in %s\n", userConfigPath)
		if err := saveDefaultConfig(userConfigPath); err != nil {
			fmt.Printf("Warning: failed to save default config: %v\n", err)
		}
	}

	// 验证配置
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// 确保必需的子目录存在
	if err := ensureSubDirs(&cfg); err != nil {
		return nil, fmt.Errorf("create sub dirs: %w", err)
	}

	return &cfg, nil
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
	v.SetDefault("logger.file.rotate_daily", false)

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

	// Memory
	v.SetDefault("memory.workspace", "")  // 空表示使用 ~/.neoray/workspace
	v.SetDefault("memory.git_enabled", true)
	v.SetDefault("memory.dream_interval", "1h")
	v.SetDefault("memory.session_ttl_minutes", 1440)  // 24小时
	v.SetDefault("memory.max_history_entries", 1000)

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
	// New tools - default to enabled for backward compatibility
	v.SetDefault("tools.find_files.enabled", true)
	v.SetDefault("tools.grep.enabled", true)
	v.SetDefault("tools.apply_patch.enabled", true)
	v.SetDefault("tools.web_search.enabled", true)
	v.SetDefault("tools.web_fetch.enabled", true)
	v.SetDefault("tools.sandbox_status.enabled", true)
	// Subagent
	v.SetDefault("tools.subagent.enabled", true)
	v.SetDefault("tools.subagent.max_concurrent", 5)
	v.SetDefault("tools.subagent.max_iterations", 10)
	v.SetDefault("tools.subagent.max_tool_result_chars", 16000)

	// Channels
	v.SetDefault("channels.websocket.enabled", true)
	v.SetDefault("channels.websocket.path", "/ws")
	v.SetDefault("channels.websocket.ping_interval", "30s")
	v.SetDefault("channels.websocket.write_wait", "10s")
	v.SetDefault("channels.websocket.pong_wait", "60s")
	v.SetDefault("channels.websocket.max_message_size", 4194304)
	v.SetDefault("channels.feishu.enabled", false)
	v.SetDefault("channels.feishu.domain", "feishu")
	v.SetDefault("channels.feishu.group_policy", "mention")
	v.SetDefault("channels.feishu.reply_to_message", true)
	v.SetDefault("channels.feishu.topic_isolation", true)
	v.SetDefault("channels.feishu.react_emoji", "THUMBSUP")
	v.SetDefault("channels.feishu.done_emoji", "")
	v.SetDefault("channels.feishu.tool_hint_prefix", "🔧")
	v.SetDefault("channels.feishu.streaming", true)

	// Security
	v.SetDefault("security.auth.enabled", true)
	v.SetDefault("security.rate_limit.enabled", true)
	v.SetDefault("security.rate_limit.requests_per_minute", 60)
	v.SetDefault("security.rate_limit.burst", 100)
	v.SetDefault("security.upload.enabled", true)
	v.SetDefault("security.upload.max_size", 10485760)
	v.SetDefault("security.upload.temp_dir", "tmp/uploads")
	v.SetDefault("security.restrict_to_workspace", true)

	// Skills
	v.SetDefault("skills.enabled", true)
	v.SetDefault("skills.builtin_skills_dir", "skills")
	v.SetDefault("skills.auto_load_always", true)

	// Web
	v.SetDefault("web.enabled", false)
}

// validate 验证配置
func validate(cfg *Config) error {
	if cfg.App.Env == "production" {
		hasAPIKey := false
		for _, provider := range cfg.LLM.Providers {
			if provider.APIKey != "" {
				hasAPIKey = true
				break
			}
		}
		if !hasAPIKey {
			return fmt.Errorf("llm api key is required in production")
		}

		// Auth must be enabled in production
		if !cfg.Security.Auth.Enabled {
			return fmt.Errorf("security.auth.enabled must be true in production")
		}

		// Secret key must not be empty or a placeholder
		secret := cfg.Security.Auth.SecretKey
		if secret == "" || secret == "your-secret-key-here" || secret == "change-me" {
			return fmt.Errorf("security.auth.secret_key must be set to a strong random value in production")
		}

		// RestrictToWorkspace should be enabled in production
		if !cfg.Security.RestrictToWorkspace {
			return fmt.Errorf("security.restrict_to_workspace must be true in production")
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
		cfg.ResolvePath("memory"),
		cfg.ResolvePath("skills"),
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
# 配置文件自动生成于: ~/.neoray/neoray.toml

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

# Anthropic API 配置示例
[llm.anthropic]
api_format = "anthropic"
api_key = ""
api_url = "https://api.anthropic.com"
model = "claude-3-sonnet-20240229"
max_tokens = 4096
temperature = 0.7
timeout = "120s"

# 记忆系统配置
[memory]
# 工作区目录（存放 SOUL.md, USER.md, memory/）
# 空表示使用 ~/.neoray
workspace = ""
# 是否启用 Git 版本控制
git_enabled = true
# Dream 处理间隔
dream_interval = "1h"
# 会话 TTL（分钟，0表示不自动归档）
session_ttl_minutes = 1440
# 最大历史条目数
max_history_entries = 1000

# 子代理系统配置
[tools.subagent]
# 是否启用子代理系统
enabled = true
# 最大并发子代理数量
max_concurrent = 5
# 每个子代理的最大迭代次数
max_iterations = 10
# 子代理工具结果的最大字符数
max_tool_result_chars = 16000

# OpenAI 兼容的 API 配置示例
[llm.openai]
api_format = "openai"
api_key = ""
api_url = "https://api.openai.com/v1"
model = "gpt-4"
max_tokens = 4096
temperature = 0.7
timeout = "120s"

# 小米/其他 OpenAI 兼容 API 示例
[llm.xiaomimimo]
api_format = "openai"
api_key = ""
api_url = "https://token-plan-cn.xiaomimimo.com/v1"
model = "mimo-v2.5-pro"
max_tokens = 4096
temperature = 0.7
timeout = "120s"
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
