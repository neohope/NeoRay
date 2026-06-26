package security

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// WorkspaceAccessMode represents the access mode for workspace.
type WorkspaceAccessMode string

const (
	WorkspaceAccessRestricted WorkspaceAccessMode = "restricted"
	WorkspaceAccessFull       WorkspaceAccessMode = "full"
)

const WorkspaceScopeMetadataKey = "workspace_scope"

var (
	trueValues  = map[string]bool{"1": true, "true": true, "yes": true, "on": true, "enabled": true}
	falseValues = map[string]bool{"0": true, "false": true, "no": true, "off": true, "disabled": true, "": true}
	providerLabels = map[string]string{
		"none":              "None",
		"unknown":           "Unknown system sandbox",
		"macos_app_sandbox": "macOS App Sandbox",
		"bwrap":             "Bubblewrap",
	}
)

// WorkspaceScopeError is raised when a requested WebUI workspace scope is invalid.
type WorkspaceScopeError struct {
	Message string
	Status  int
}

func (e *WorkspaceScopeError) Error() string {
	return e.Message
}

// WorkspaceSandboxStatus represents the resolved workspace sandbox state for runtime display and tooling.
type WorkspaceSandboxStatus struct {
	RestrictToWorkspace bool   `json:"restrict_to_workspace"`
	WorkspaceRoot       string `json:"workspace_root"`
	Level               string `json:"level"`
	Enforced            bool   `json:"enforced"`
	Provider            string `json:"provider"`
	ProviderLabel       string `json:"provider_label"`
	Summary             string `json:"summary"`
}

// AsDict returns the sandbox status as a map.
func (s *WorkspaceSandboxStatus) AsDict() map[string]interface{} {
	return map[string]interface{}{
		"restrict_to_workspace": s.RestrictToWorkspace,
		"workspace_root":        s.WorkspaceRoot,
		"level":                 s.Level,
		"enforced":              s.Enforced,
		"provider":              s.Provider,
		"provider_label":        s.ProviderLabel,
		"summary":               s.Summary,
	}
}

// WorkspaceScope represents the effective project root and access mode for one agent turn.
type WorkspaceScope struct {
	ProjectPath         string                `json:"project_path"`
	AccessMode          WorkspaceAccessMode   `json:"access_mode"`
	RestrictToWorkspace bool                  `json:"restrict_to_workspace"`
	SandboxStatus       *WorkspaceSandboxStatus `json:"sandbox_status"`
	SourceChannel       string                `json:"source_channel,omitempty"`
}

// ProjectName returns the name of the project.
func (s *WorkspaceScope) ProjectName() string {
	return filepath.Base(s.ProjectPath)
}

// Metadata returns the scope as metadata.
func (s *WorkspaceScope) Metadata() map[string]string {
	return map[string]string{
		"project_path": s.ProjectPath,
		"access_mode":  string(s.AccessMode),
	}
}

// Payload returns the scope as a payload map.
func (s *WorkspaceScope) Payload() map[string]interface{} {
	result := map[string]interface{}{
		"project_path":         s.ProjectPath,
		"access_mode":          string(s.AccessMode),
		"project_name":         s.ProjectName(),
		"restrict_to_workspace": s.RestrictToWorkspace,
	}
	if s.SandboxStatus != nil {
		result["sandbox_status"] = s.SandboxStatus.AsDict()
	}
	return result
}

// ToolWorkspace represents the workspace policy resolved for a tool call.
type ToolWorkspace struct {
	ProjectPath         string
	RestrictToWorkspace bool
	Scope               *WorkspaceScope
}

// AllowedRoot returns the allowed root path if restriction is enabled.
func (t *ToolWorkspace) AllowedRoot() string {
	if t.RestrictToWorkspace && t.ProjectPath != "" {
		return t.ProjectPath
	}
	return ""
}

// WorkspaceScopeResolver resolves the effective workspace scope at an agent turn boundary.
type WorkspaceScopeResolver struct {
	DefaultWorkspace          string
	DefaultRestrictToWorkspace bool
	ScopedChannel             string
}

// NewWorkspaceScopeResolver creates a new WorkspaceScopeResolver.
func NewWorkspaceScopeResolver(
	defaultWorkspace string,
	defaultRestrictToWorkspace bool,
	scopedChannel string,
) *WorkspaceScopeResolver {
	if scopedChannel == "" {
		scopedChannel = "websocket"
	}
	return &WorkspaceScopeResolver{
		DefaultWorkspace:          defaultWorkspace,
		DefaultRestrictToWorkspace: defaultRestrictToWorkspace,
		ScopedChannel:             scopedChannel,
	}
}

// SandboxStatus returns the default sandbox status.
func (r *WorkspaceScopeResolver) SandboxStatus() *WorkspaceSandboxStatus {
	return r.Default().SandboxStatus
}

// Default returns the default workspace scope.
func (r *WorkspaceScopeResolver) Default() *WorkspaceScope {
	return DefaultWorkspaceScope(r.DefaultWorkspace, r.DefaultRestrictToWorkspace, "")
}

// ForTurn returns the workspace scope for an agent turn.
func (r *WorkspaceScopeResolver) ForTurn(
	channel string,
	messageMetadata map[string]interface{},
	sessionMetadata map[string]interface{},
) *WorkspaceScope {
	if channel != r.ScopedChannel {
		return r.Default()
	}
	return ResolveEffectiveWorkspaceScope(
		messageMetadata,
		sessionMetadata,
		r.DefaultWorkspace,
		r.DefaultRestrictToWorkspace,
		channel,
	)
}

// PersistMessageScope persists the workspace scope from a message to session metadata.
func (r *WorkspaceScopeResolver) PersistMessageScope(
	sessionMetadata map[string]interface{},
	messageMetadata map[string]interface{},
	channel string,
) {
	if channel != r.ScopedChannel {
		return
	}
	raw, ok := messageMetadata[WorkspaceScopeMetadataKey]
	if !ok {
		return
	}
	scopeMap, ok := raw.(map[string]interface{})
	if !ok {
		return
	}
	sessionMetadata[WorkspaceScopeMetadataKey] = copyMap(scopeMap)
}

// CurrentWorkspaceScope holds the current workspace scope in a thread-safe manner.
var (
	currentWorkspaceScope *WorkspaceScope
	scopeMu               sync.RWMutex
)

// WorkspaceSandboxStatus returns how workspace restriction is enforced in the current host.
func WorkspaceSandboxStatusFromConfig(
	restrictToWorkspace bool,
	workspace string,
	environ map[string]string,
) *WorkspaceSandboxStatus {
	resolvedWorkspace, _ := ResolvePath(workspace, "", false)
	provider := envSystemProvider(environ)

	if !restrictToWorkspace {
		return &WorkspaceSandboxStatus{
			RestrictToWorkspace: false,
			WorkspaceRoot:       resolvedWorkspace,
			Level:               "off",
			Enforced:            false,
			Provider:            "none",
			ProviderLabel:       getProviderLabel("none"),
			Summary:             "Workspace restriction is disabled.",
		}
	}

	if provider != "" {
		label := getProviderLabel(provider)
		return &WorkspaceSandboxStatus{
			RestrictToWorkspace: true,
			WorkspaceRoot:       resolvedWorkspace,
			Level:               "system",
			Enforced:            true,
			Provider:            provider,
			ProviderLabel:       label,
			Summary:             "Workspace restriction is system-enforced by " + label + ".",
		}
	}

	return &WorkspaceSandboxStatus{
		RestrictToWorkspace: true,
		WorkspaceRoot:       resolvedWorkspace,
		Level:               "application",
		Enforced:            false,
		Provider:            "none",
		ProviderLabel:       getProviderLabel("none"),
		Summary:             "Workspace restriction uses NeoRay application-level guards.",
	}
}

// DefaultAccessMode returns the default access mode based on restrict_to_workspace.
func DefaultAccessMode(restrictToWorkspace bool) WorkspaceAccessMode {
	if restrictToWorkspace {
		return WorkspaceAccessRestricted
	}
	return WorkspaceAccessFull
}

// BuildWorkspaceScope builds a workspace scope from parameters.
func BuildWorkspaceScope(
	projectPath string,
	accessMode string,
	sourceChannel string,
) (*WorkspaceScope, error) {
	mode := normalizeAccessMode(accessMode)
	root, err := ResolvePath(projectPath, "", false)
	if err != nil {
		root = projectPath
	}
	restrict := mode == WorkspaceAccessRestricted

	return &WorkspaceScope{
		ProjectPath:         root,
		AccessMode:          mode,
		RestrictToWorkspace: restrict,
		SandboxStatus:       WorkspaceSandboxStatusFromConfig(restrict, root, nil),
		SourceChannel:       sourceChannel,
	}, nil
}

// DefaultWorkspaceScope returns the default workspace scope.
func DefaultWorkspaceScope(
	workspace string,
	restrictToWorkspace bool,
	sourceChannel string,
) *WorkspaceScope {
	scope, _ := BuildWorkspaceScope(
		workspace,
		string(DefaultAccessMode(restrictToWorkspace)),
		sourceChannel,
	)
	return scope
}

// ValidateWorkspaceScopePayload validates a client-requested workspace scope.
func ValidateWorkspaceScopePayload(
	raw interface{},
	defaultWorkspace string,
	defaultRestrictToWorkspace bool,
	sourceChannel string,
) (*WorkspaceScope, error) {
	if raw == nil {
		return DefaultWorkspaceScope(defaultWorkspace, defaultRestrictToWorkspace, sourceChannel), nil
	}

	scopeMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, &WorkspaceScopeError{Message: "workspace_scope must be an object", Status: 400}
	}

	// Extract project_path
	var rawPath string
	if pathVal, ok := scopeMap["project_path"]; ok && pathVal != "" {
		rawPath, _ = pathVal.(string)
	} else if pathVal, ok := scopeMap["path"]; ok && pathVal != "" {
		rawPath, _ = pathVal.(string)
	} else {
		rawPath, _ = ResolvePath(defaultWorkspace, "", false)
	}

	if rawPath == "" {
		rawPath, _ = ResolvePath(defaultWorkspace, "", false)
	}

	if strings.Contains(rawPath, "\x00") {
		return nil, &WorkspaceScopeError{Message: "project_path contains invalid characters", Status: 400}
	}

	// Resolve project path
	project := os.ExpandEnv(rawPath)
	if strings.HasPrefix(project, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			project = filepath.Join(home, project[1:])
		}
	}

	if !filepath.IsAbs(project) {
		return nil, &WorkspaceScopeError{Message: "project_path must be absolute", Status: 400}
	}

	projectAbs, err := filepath.Abs(project)
	if err != nil {
		projectAbs = project
	}

	// Check if it's a directory
	if info, err := os.Stat(projectAbs); err != nil || !info.IsDir() {
		// Don't fail if directory doesn't exist yet, just use the path
	}

	// Extract access_mode
	var rawMode string
	if modeVal, ok := scopeMap["access_mode"]; ok && modeVal != nil {
		rawMode, _ = modeVal.(string)
	}
	if rawMode == "" {
		rawMode = string(DefaultAccessMode(defaultRestrictToWorkspace))
	}

	return BuildWorkspaceScope(projectAbs, rawMode, sourceChannel)
}

// WorkspaceScopeFromMetadata resolves persisted metadata, falling back safely.
func WorkspaceScopeFromMetadata(
	metadata map[string]interface{},
	defaultWorkspace string,
	defaultRestrictToWorkspace bool,
	sourceChannel string,
) *WorkspaceScope {
	if metadata == nil {
		return DefaultWorkspaceScope(defaultWorkspace, defaultRestrictToWorkspace, sourceChannel)
	}

	scope, err := ValidateWorkspaceScopePayload(
		metadata[WorkspaceScopeMetadataKey],
		defaultWorkspace,
		defaultRestrictToWorkspace,
		sourceChannel,
	)
	if err != nil {
		return DefaultWorkspaceScope(defaultWorkspace, defaultRestrictToWorkspace, sourceChannel)
	}
	return scope
}

// ResolveEffectiveWorkspaceScope resolves the effective workspace scope.
func ResolveEffectiveWorkspaceScope(
	messageMetadata map[string]interface{},
	sessionMetadata map[string]interface{},
	defaultWorkspace string,
	defaultRestrictToWorkspace bool,
	sourceChannel string,
) *WorkspaceScope {
	if messageMetadata != nil {
		if _, ok := messageMetadata[WorkspaceScopeMetadataKey]; ok {
			return WorkspaceScopeFromMetadata(
				messageMetadata,
				defaultWorkspace,
				defaultRestrictToWorkspace,
				sourceChannel,
			)
		}
	}
	return WorkspaceScopeFromMetadata(
		sessionMetadata,
		defaultWorkspace,
		defaultRestrictToWorkspace,
		sourceChannel,
	)
}

// BindWorkspaceScope sets the current workspace scope.
func BindWorkspaceScope(scope *WorkspaceScope) {
	scopeMu.Lock()
	defer scopeMu.Unlock()
	currentWorkspaceScope = scope
}

// ResetWorkspaceScope resets the current workspace scope.
func ResetWorkspaceScope() {
	scopeMu.Lock()
	defer scopeMu.Unlock()
	currentWorkspaceScope = nil
}

// CurrentWorkspaceScope returns the current workspace scope.
func CurrentWorkspaceScope() *WorkspaceScope {
	scopeMu.RLock()
	defer scopeMu.RUnlock()
	return currentWorkspaceScope
}

// CurrentToolWorkspace returns the workspace/access policy for the current tool call.
func CurrentToolWorkspace(
	defaultWorkspace string,
	restrictToWorkspace bool,
	sandboxRestrictsWorkspace bool,
) *ToolWorkspace {
	scope := CurrentWorkspaceScope()

	var projectPath string
	if scope != nil {
		projectPath = scope.ProjectPath
	} else if defaultWorkspace != "" {
		projectPath, _ = ResolvePath(defaultWorkspace, "", false)
	}

	var restrict bool
	if scope != nil {
		restrict = scope.RestrictToWorkspace
	} else {
		restrict = restrictToWorkspace
	}
	restrict = restrict || sandboxRestrictsWorkspace

	return &ToolWorkspace{
		ProjectPath:         projectPath,
		RestrictToWorkspace: restrict,
		Scope:               scope,
	}
}

// CurrentScopeAllowsLoopback returns true when the current WebUI Full Access turn may touch loopback URLs.
func CurrentScopeAllowsLoopback(enabled bool) bool {
	scope := CurrentWorkspaceScope()
	return enabled &&
		scope != nil &&
		scope.SourceChannel == "websocket" &&
		scope.AccessMode == WorkspaceAccessFull &&
		!scope.RestrictToWorkspace
}

// Helper functions

func envSystemProvider(environ map[string]string) string {
	var env map[string]string
	if environ != nil {
		env = environ
	} else {
		env = make(map[string]string)
		for _, e := range os.Environ() {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}
	}

	explicitProvider := env["NEORAY_WORKSPACE_SANDBOX_PROVIDER"]
	enforced := env["NEORAY_WORKSPACE_SANDBOX_ENFORCED"]
	compatibility := env["NEORAY_SANDBOX_ENFORCED"]

	marker := enforced
	if marker == "" {
		marker = compatibility
	}
	if marker == "" {
		return ""
	}

	normalizedMarker := strings.ToLower(strings.TrimSpace(marker))
	if falseValues[normalizedMarker] {
		return ""
	}
	if trueValues[normalizedMarker] {
		return normalizeProvider(explicitProvider)
	}
	return normalizeProvider(marker)
}

func normalizeProvider(value string) string {
	if value == "" {
		return "unknown"
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func getProviderLabel(provider string) string {
	if label, ok := providerLabels[provider]; ok {
		return label
	}
	return strings.Title(strings.ReplaceAll(provider, "_", " "))
}

func normalizeAccessMode(value string) WorkspaceAccessMode {
	mode := strings.ToLower(strings.TrimSpace(value))
	mode = strings.ReplaceAll(mode, "_", "-")
	if mode == "restrict" {
		mode = "restricted"
	}
	if mode == "full-access" {
		mode = "full"
	}
	if mode == "restricted" || mode == "full" {
		return WorkspaceAccessMode(mode)
	}
	return WorkspaceAccessRestricted
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}
