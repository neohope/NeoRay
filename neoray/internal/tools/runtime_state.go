package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// RuntimeState 运行时状态
type RuntimeState struct {
	mu                  sync.RWMutex
	maxIterations       int
	contextWindowTokens int
	model               string
	modelPreset         string
	workspace           string
	providerRetryMode   string
	maxToolResultChars  int
	webConfigEnable     bool
	webConfig           map[string]interface{}
	workspaceSandbox    string
	subagents           map[string]interface{}
	lastUsage           TokenUsage
	currentIteration    int
	runtimeVars         map[string]interface{}
}

// TokenUsage Token 使用统计
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// NewRuntimeState 创建运行时状态
func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		maxIterations:       10,
		contextWindowTokens: 128000,
		workspace:           "",
		runtimeVars:         make(map[string]interface{}),
	}
}

// GetMaxIterations 获取最大迭代次数
func (s *RuntimeState) GetMaxIterations() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxIterations
}

// SetMaxIterations 设置最大迭代次数
func (s *RuntimeState) SetMaxIterations(val int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxIterations = val
}

// GetContextWindowTokens 获取上下文窗口大小
func (s *RuntimeState) GetContextWindowTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.contextWindowTokens
}

// SetContextWindowTokens 设置上下文窗口大小
func (s *RuntimeState) SetContextWindowTokens(val int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextWindowTokens = val
}

// GetModel 获取模型
func (s *RuntimeState) GetModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.model
}

// SetModel 设置模型
func (s *RuntimeState) SetModel(val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = val
	s.modelPreset = ""
}

// GetModelPreset 获取模型预设
func (s *RuntimeState) GetModelPreset() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelPreset
}

// SetModelPreset 设置模型预设
func (s *RuntimeState) SetModelPreset(val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modelPreset = val
}

// GetWorkspace 获取工作区
func (s *RuntimeState) GetWorkspace() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workspace
}

// SetWorkspace 设置工作区
func (s *RuntimeState) SetWorkspace(val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspace = val
}

// GetLastUsage 获取最后一次 token 使用
func (s *RuntimeState) GetLastUsage() TokenUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUsage
}

// SetLastUsage 设置最后一次 token 使用
func (s *RuntimeState) SetLastUsage(prompt, completion int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUsage = TokenUsage{PromptTokens: prompt, CompletionTokens: completion}
}

// GetCurrentIteration 获取当前迭代
func (s *RuntimeState) GetCurrentIteration() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentIteration
}

// SetCurrentIteration 设置当前迭代
func (s *RuntimeState) SetCurrentIteration(val int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentIteration = val
}

// GetRuntimeVar 获取运行时变量
func (s *RuntimeState) GetRuntimeVar(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.runtimeVars[key]
	return val, ok
}

// SetRuntimeVar 设置运行时变量
func (s *RuntimeState) SetRuntimeVar(key string, val interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeVars[key] = val
}

// GetAllRuntimeVars 获取所有运行时变量
func (s *RuntimeState) GetAllRuntimeVars() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vars := make(map[string]interface{})
	for k, v := range s.runtimeVars {
		vars[k] = v
	}
	return vars
}

// SetWebConfig 设置 Web 配置
func (s *RuntimeState) SetWebConfig(enable bool, config map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webConfigEnable = enable
	if config != nil {
		s.webConfig = config
	}
}

// IsWebConfigEnabled Web 配置是否启用
func (s *RuntimeState) IsWebConfigEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.webConfigEnable
}

// GetWebConfig 获取 Web 配置
func (s *RuntimeState) GetWebConfig() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	config := make(map[string]interface{})
	for k, v := range s.webConfig {
		config[k] = v
	}
	return config
}

// ToMap 转换为 Map
func (s *RuntimeState) ToMap() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"max_iterations":        s.maxIterations,
		"context_window_tokens": s.contextWindowTokens,
		"model":                 s.model,
		"model_preset":          s.modelPreset,
		"workspace":             s.workspace,
		"provider_retry_mode":   s.providerRetryMode,
		"max_tool_result_chars": s.maxToolResultChars,
		"web_config": map[string]interface{}{
			"enable": s.webConfigEnable,
		},
		"workspace_sandbox": s.workspaceSandbox,
		"subagents":         s.subagents,
		"_last_usage":       s.lastUsage,
		"_current_iteration": s.currentIteration,
		"scratchpad":        s.runtimeVars,
	}
}

// MarshalJSON JSON 序列化
func (s *RuntimeState) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.ToMap())
}

// FormatValue 格式化值显示
func FormatValue(val interface{}, key string) string {
	switch v := val.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return fmt.Sprintf("%s: {}", key)
		}
		// 检查是否是 subagent status map
		if key == "subagents" || key == "scratchpad" {
			items := make([]string, 0, len(v))
			for k, item := range v {
				items = append(items, fmt.Sprintf("  %s: %v", k, item))
			}
			return fmt.Sprintf("%s:\n%s", key, strings.Join(items, "\n"))
		}
		// 小对象，显示内容
		if len(v) <= 8 {
			items := make([]string, 0, len(v))
			for k, item := range v {
				// 跳过敏感字段
				if isSensitiveKey(k) {
					continue
				}
				items = append(items, fmt.Sprintf("%s: %v", k, item))
			}
			return fmt.Sprintf("%s: {%s}", key, strings.Join(items, ", "))
		}
		keys := make([]string, 0, 15)
		for k := range v {
			if len(keys) >= 15 {
				break
			}
			keys = append(keys, k)
		}
		suffix := ""
		if len(v) > 15 {
			suffix = ", ..."
		}
		return fmt.Sprintf("%s: {%s%s}", key, strings.Join(keys, ", "), suffix)
	case []interface{}:
		if len(v) > 20 {
			return fmt.Sprintf("%s: [%d items]", key, len(v))
		}
		return fmt.Sprintf("%s: %v", key, v)
	case TokenUsage:
		return fmt.Sprintf("%s: {prompt_tokens: %d, completion_tokens: %d}", key, v.PromptTokens, v.CompletionTokens)
	default:
		return fmt.Sprintf("%s: %v", key, v)
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	sensitiveKeys := []string{"api_key", "secret", "password", "token", "credential", "private_key", "access_token", "refresh_token", "auth"}
	for _, s := range sensitiveKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
