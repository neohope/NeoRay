package tools

import (
	"context"
	"encoding/json"

	"neoray/internal/config"
	"neoray/internal/security"
)

// SandboxStatusTool 沙箱状态工具
type SandboxStatusTool struct {
	cfg *config.Config
}

// NewSandboxStatusTool 创建沙箱状态工具
func NewSandboxStatusTool(cfg *config.Config) *SandboxStatusTool {
	return &SandboxStatusTool{
		cfg: cfg,
	}
}

// Name 工具名称
func (t *SandboxStatusTool) Name() string {
	return "sandbox_status"
}

// Description 工具描述
func (t *SandboxStatusTool) Description() string {
	return "Get the current sandbox and workspace security status"
}

// Parameters 参数定义
func (t *SandboxStatusTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{}, []string{})
}

// Execute 执行工具
func (t *SandboxStatusTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	workspace := t.cfg.ResolvePath(t.cfg.Tools.Shell.WorkingDir)
	sandboxStatus := security.WorkspaceSandboxStatusFromConfig(
		t.cfg.Security.RestrictToWorkspace,
		workspace,
		nil,
	)

	result := map[string]any{
		"success": true,
		"status":  sandboxStatus.AsDict(),
		"config": map[string]any{
			"sandbox":                      t.cfg.Tools.Shell.Sandbox,
			"restrict_to_workspace":       t.cfg.Security.RestrictToWorkspace,
			"webui_allow_local_service_access": t.cfg.Security.WebUIAllowLocalServiceAccess,
			"ssrf_whitelist":              t.cfg.Security.SSRFWhitelist,
			"workspace":                   workspace,
		},
	}

	// 如果有当前的工作区范围，也包含它
	if currentScope := security.WorkspaceScopeFromContext(ctx); currentScope != nil {
		result["current_scope"] = currentScope.Payload()
	}

	res, _ := json.Marshal(result)
	return res, nil
}
