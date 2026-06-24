package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ReflectionTool reflection 工具
type ReflectionTool struct {
	runtimeState  *RuntimeState
	modifyAllowed bool
}

// ReflectionArgs reflection 参数
type ReflectionArgs struct {
	Action string      `json:"action"` // "check" 或 "set"
	Key    string      `json:"key,omitempty"`
	Value  interface{} `json:"value,omitempty"`
}

// NewReflectionTool 创建 reflection 工具
func NewReflectionTool(state *RuntimeState, modifyAllowed bool) *ReflectionTool {
	return &ReflectionTool{
		runtimeState:  state,
		modifyAllowed: modifyAllowed,
	}
}

// Name 返回工具名称
func (t *ReflectionTool) Name() string {
	return "reflection"
}

// Description 返回工具描述
func (t *ReflectionTool) Description() string {
	base := "Check and set your own runtime state. Actions: check, set.\n"
	base += "- check (no key): full config overview — start here.\n"
	base += "- check (key): drill into a value. Dot-paths allowed (e.g., \"_last_usage.prompt_tokens\", \"web_config.enable\").\n"
	base += "- set (key, value): change config or store notes in your scratchpad. Scratchpad keys persist across turns but not restarts.\n"
	base += "Key values: _current_iteration (current progress), max_iterations - _current_iteration = remaining iterations.\n"
	base += "Note: web_config is readable but read-only.\n\n"
	base += "When to use:\n"
	base += "- User asks about your model, settings, or token usage → check that key.\n"
	base += "- A tool fails or behaves unexpectedly → check the related config to diagnose.\n"
	base += "- User asks you to remember a preference for this session → set to store it in your scratchpad.\n"
	base += "- About to start a large task → check context_window_tokens and max_iterations first."

	if !t.modifyAllowed {
		base += "\nREAD-ONLY MODE: set is disabled."
	} else {
		base += "\nIMPORTANT: Before setting state, predict the potential impact. If the operation could cause crashes or instability (e.g., changing model), warn the user first."
	}
	return base
}

// Parameters 返回参数定义
func (t *ReflectionTool) Parameters() json.RawMessage {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"check", "set"},
				"description": "Action to perform",
			},
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Dot-path for check/set. Examples: \"max_iterations\", \"workspace\", \"provider_retry_mode\". For check without key, shows all config values.",
			},
			"value": map[string]interface{}{
				"description": "New value (for set). Type must match target (int for max_iterations/context_window_tokens, str for model).",
			},
		},
		"required": []string{"action"},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return data
}

// Execute 执行工具
func (t *ReflectionTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input ReflectionArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	switch input.Action {
	case "check":
		return t.handleCheck(input.Key)
	case "set":
		if !t.modifyAllowed {
			return json.Marshal("Error: set is disabled (tools.reflection.allow_set is false)")
		}
		return t.handleSet(input.Key, input.Value)
	default:
		return json.Marshal(fmt.Sprintf("Error: Unknown action '%s'", input.Action))
	}
}

func (t *ReflectionTool) handleCheck(key string) (json.RawMessage, error) {
	if key == "" {
		return t.inspectAll()
	}

	// 检查是否是 scratchpad 别名
	if key == "scratchpad" {
		vars := t.runtimeState.GetAllRuntimeVars()
		if len(vars) == 0 {
			return json.Marshal("scratchpad is empty")
		}
		return json.Marshal(FormatValue(vars, "scratchpad"))
	}

	// 尝试解析点路径
	val, err := t.resolvePath(key)
	if err == nil {
		return json.Marshal(FormatValue(val, key))
	}

	// 如果不是路径，尝试在 runtime_vars 中查找
	if !strings.Contains(key, ".") {
		if val, ok := t.runtimeState.GetRuntimeVar(key); ok {
			return json.Marshal(FormatValue(val, key))
		}
	}

	return json.Marshal(fmt.Sprintf("Error: %s", err))
}

func (t *ReflectionTool) inspectAll() (json.RawMessage, error) {
	state := t.runtimeState
	var parts []string

	// 限制配置字段
	restrictedKeys := []string{
		"max_iterations",
		"context_window_tokens",
		"model",
		"model_preset",
		"workspace",
		"provider_retry_mode",
		"max_tool_result_chars",
		"_current_iteration",
		"web_config",
		"workspace_sandbox",
		"subagents",
		"_last_usage",
		"scratchpad",
	}

	stateMap := state.ToMap()
	for _, k := range restrictedKeys {
		if val, ok := stateMap[k]; ok {
			parts = append(parts, FormatValue(val, k))
		}
	}

	return json.Marshal(strings.Join(parts, "\n"))
}

func (t *ReflectionTool) handleSet(key string, value interface{}) (json.RawMessage, error) {
	if key == "" {
		return json.Marshal("Error: key cannot be empty")
	}

	protectedKeys := map[string]bool{
		"bus": true, "provider": true, "_running": true, "tools": true,
		"_runtime_vars": true, "runner": true, "sessions": true, "consolidation": true,
		"dream": true, "auto_compact": true, "context": true, "commands": true,
		"_pending_queues": true, "_session_locks": true, "_active_tasks": true,
		"_background_tasks": true, "restrict_to_workspace": true, "channels_config": true,
		"_concurrency_gate": true, "_unified_session": true, "_extra_hooks": true,
	}
	readOnlyKeys := map[string]bool{
		"subagents": true, "_current_iteration": true, "web_config": true,
		"exec_config": true, "workspace_sandbox": true,
	}
	deniedKeys := map[string]bool{
		"__class__": true, "__dict__": true, "__bases__": true, "__subclasses__": true,
		"__mro__": true, "__init__": true, "__new__": true, "__reduce__": true,
		"__getstate__": true, "__setstate__": true, "__del__": true, "__call__": true,
		"__getattr__": true, "__setattr__": true, "__delattr__": true, "__code__": true,
		"__globals__": true, "func_globals": true, "func_code": true, "__wrapped__": true,
		"__closure__": true,
	}
	isDenied := func(k string) bool {
		return deniedKeys[k] || strings.HasPrefix(k, "__")
	}

	if protectedKeys[key] || isDenied(key) {
		return json.Marshal(fmt.Sprintf("Error: '%s' is protected and cannot be modified", key))
	}

	if readOnlyKeys[key] {
		return json.Marshal(fmt.Sprintf("Error: '%s' is read-only and cannot be modified", key))
	}

	if strings.Contains(key, ".") {
		parts := strings.SplitN(key, ".", 2)
		parentKey := parts[0]
		leafKey := parts[1]

		if isDenied(leafKey) || isSensitiveKey(leafKey) {
			return json.Marshal(fmt.Sprintf("Error: '%s' is not accessible", leafKey))
		}

		if parentKey == "scratchpad" {
			t.runtimeState.SetRuntimeVar(leafKey, value)
			return json.Marshal(fmt.Sprintf("Set scratchpad.%s = %v", leafKey, value))
		}

		return json.Marshal(fmt.Sprintf("Error: nested modification for '%s' is not supported", parentKey))
	}

	restrictedEditKeys := map[string]bool{
		"max_iterations": true, "context_window_tokens": true, "model": true,
	}
	if restrictedEditKeys[key] {
		return t.handleSetRestricted(key, value)
	}

	return t.handleSetFree(key, value)
}

func (t *ReflectionTool) handleSetRestricted(key string, value interface{}) (json.RawMessage, error) {
	switch key {
	case "max_iterations":
		var intVal int
		switch v := value.(type) {
		case float64:
			intVal = int(v)
		case int:
			intVal = v
		default:
			return json.Marshal(fmt.Sprintf("Error: '%s' must be int, got %T", key, value))
		}
		if intVal < 1 || intVal > 100 {
			return json.Marshal(fmt.Sprintf("Error: '%s' must be between 1 and 100", key))
		}
		old := t.runtimeState.GetMaxIterations()
		t.runtimeState.SetMaxIterations(intVal)
		return json.Marshal(fmt.Sprintf("Set %s = %d (was %d)", key, intVal, old))

	case "context_window_tokens":
		var intVal int
		switch v := value.(type) {
		case float64:
			intVal = int(v)
		case int:
			intVal = v
		default:
			return json.Marshal(fmt.Sprintf("Error: '%s' must be int, got %T", key, value))
		}
		if intVal < 4096 || intVal > 1000000 {
			return json.Marshal(fmt.Sprintf("Error: '%s' must be between 4096 and 1000000", key))
		}
		old := t.runtimeState.GetContextWindowTokens()
		t.runtimeState.SetContextWindowTokens(intVal)
		return json.Marshal(fmt.Sprintf("Set %s = %d (was %d)", key, intVal, old))

	case "model":
		strVal, ok := value.(string)
		if !ok {
			return json.Marshal(fmt.Sprintf("Error: '%s' must be string, got %T", key, value))
		}
		if len(strVal) == 0 {
			return json.Marshal(fmt.Sprintf("Error: '%s' must be at least 1 character", key))
		}
		old := t.runtimeState.GetModel()
		t.runtimeState.SetModel(strVal)
		if old == "" {
			return json.Marshal(fmt.Sprintf("Set %s = %q", key, strVal))
		}
		return json.Marshal(fmt.Sprintf("Set %s = %q (was %q)", key, strVal, old))

	default:
		return json.Marshal(fmt.Sprintf("Error: '%s' is not a configurable restricted key", key))
	}
}

func (t *ReflectionTool) handleSetFree(key string, value interface{}) (json.RawMessage, error) {
	stateMap := t.runtimeState.ToMap()
	if oldVal, hasKey := stateMap[key]; hasKey {
		switch oldVal.(type) {
		case string:
			if _, ok := value.(string); !ok {
				return json.Marshal(fmt.Sprintf("Error: '%s' expects string, got %T", key, value))
			}
		case int, int64, float64:
		case bool:
			if _, ok := value.(bool); !ok {
				return json.Marshal(fmt.Sprintf("Error: '%s' expects bool, got %T", key, value))
			}
		}
		return json.Marshal(fmt.Sprintf("Error: '%s' is not directly modifiable (use scratchpad)", key))
	}

	if err := t.validateJSONSafe(value, 0); err != nil {
		return json.Marshal(fmt.Sprintf("Error: %s", err))
	}

	vars := t.runtimeState.GetAllRuntimeVars()
	if _, exists := vars[key]; !exists && len(vars) >= 64 {
		return json.Marshal("Error: scratchpad is full (max 64 keys). Remove unused keys first.")
	}

	oldVal, hasOld := t.runtimeState.GetRuntimeVar(key)
	t.runtimeState.SetRuntimeVar(key, value)

	if hasOld {
		return json.Marshal(fmt.Sprintf("Set scratchpad.%s = %v (was %v)", key, value, oldVal))
	}
	return json.Marshal(fmt.Sprintf("Set scratchpad.%s = %v", key, value))
}

func (t *ReflectionTool) resolvePath(path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid path")
	}

	protectedKeys := map[string]bool{
		"bus": true, "provider": true, "_running": true, "tools": true,
		"_runtime_vars": true, "runner": true, "sessions": true, "consolidation": true,
		"dream": true, "auto_compact": true, "context": true, "commands": true,
		"_pending_queues": true, "_session_locks": true, "_active_tasks": true,
		"_background_tasks": true, "restrict_to_workspace": true, "channels_config": true,
		"_concurrency_gate": true, "_unified_session": true, "_extra_hooks": true,
	}
	deniedKeys := map[string]bool{
		"__class__": true, "__dict__": true, "__bases__": true, "__subclasses__": true,
		"__mro__": true, "__init__": true, "__new__": true, "__reduce__": true,
		"__getstate__": true, "__setstate__": true, "__del__": true, "__call__": true,
		"__getattr__": true, "__setattr__": true, "__delattr__": true, "__code__": true,
		"__globals__": true, "func_globals": true, "func_code": true, "__wrapped__": true,
		"__closure__": true,
	}
	isDenied := func(k string) bool {
		return deniedKeys[k] || strings.HasPrefix(k, "__")
	}

	stateMap := t.runtimeState.ToMap()
	var current interface{} = stateMap

	for _, part := range parts {
		if isDenied(part) || isSensitiveKey(part) {
			return nil, fmt.Errorf("'%s' is not accessible", part)
		}
		if protectedKeys[part] {
			continue
		}

		switch m := current.(type) {
		case map[string]interface{}:
			if val, ok := m[part]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("'%s' not found", part)
			}
		default:
			return nil, fmt.Errorf("'%s' not found", part)
		}
	}

	return current, nil
}

func (t *ReflectionTool) validateJSONSafe(value interface{}, depth int) error {
	if depth > 10 {
		return fmt.Errorf("value nesting too deep (max 10 levels)")
	}
	switch v := value.(type) {
	case string, int, int64, float64, bool, nil:
		return nil
	case []interface{}:
		for _, item := range v {
			if err := t.validateJSONSafe(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]interface{}:
		for _, val := range v {
			if err := t.validateJSONSafe(val, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported type %T", value)
	}
}