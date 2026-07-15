package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"neoray/internal/config"
	"neoray/internal/logger"
	"neoray/internal/security"
)

// FileSystemTool 文件系统工具
type FileSystemTool struct {
	cfg         *config.Config
	workspace   string
	fileStates  *FileStates
}

// NewFileSystemTool 创建文件系统工具
func NewFileSystemTool(cfg *config.Config) *FileSystemTool {
	workspace := resolveWorkspacePath(cfg)
	return &FileSystemTool{
		cfg:         cfg,
		workspace:   workspace,
		fileStates:  NewFileStates(),
	}
}

// NewFileSystemToolWithFileStates 创建带 FileStates 的文件系统工具
func NewFileSystemToolWithFileStates(cfg *config.Config, fileStates *FileStates) *FileSystemTool {
	workspace := resolveWorkspacePath(cfg)
	return &FileSystemTool{
		cfg:         cfg,
		workspace:   workspace,
		fileStates:  fileStates,
	}
}

// resolveWorkspacePath 解析 workspace 路径，评估符号链接以防止 TOCTOU 攻击
func resolveWorkspacePath(cfg *config.Config) string {
	rawPath := cfg.ResolvePath("workspace")
	resolved, err := filepath.EvalSymlinks(rawPath)
	if err != nil {
		// 路径可能不存在，尝试解析父目录
		parent := filepath.Dir(rawPath)
		resolvedParent, perr := filepath.EvalSymlinks(parent)
		if perr != nil {
			// 父目录也无法解析，返回原始路径（后续验证会捕获问题）
			return rawPath
		}
		return filepath.Join(resolvedParent, filepath.Base(rawPath))
	}
	return resolved
}

// SetFileStates 设置 FileStates
func (t *FileSystemTool) SetFileStates(fileStates *FileStates) {
	t.fileStates = fileStates
}

// GetFileStates 获取 FileStates
func (t *FileSystemTool) GetFileStates() *FileStates {
	return t.fileStates
}

// Name 工具名称
func (t *FileSystemTool) Name() string {
	return "filesystem"
}

// Description 工具描述
func (t *FileSystemTool) Description() string {
	return "Read, write, and manage files in the workspace"
}

// Parameters 参数定义
func (t *FileSystemTool) Parameters() json.RawMessage {
	return ObjectParam(map[string]any{
		"action": map[string]any{
			"type":        "string",
			"description": "Action to perform: read, write, list, delete",
			"enum":        []string{"read", "write", "list", "delete"},
		},
		"path": StringParam("File or directory path (relative to workspace)"),
		"content": StringParam("File content for write action"),
		"overwrite": BooleanParam("Whether to overwrite existing file (default: false)"),
	}, []string{"action", "path"})
}

// Execute 执行工具
func (t *FileSystemTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Action    string `json:"action"`
		Path      string `json:"path"`
		Content   string `json:"content,omitempty"`
		Overwrite bool   `json:"overwrite,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// 安全检查：确保路径在 workspace 内
	// 使用 EvalSymlinks 解析输入路径，防止符号链接指向 workspace 外部
	inputPath := filepath.Join(t.workspace, params.Path)
	resolvedInput, err := filepath.EvalSymlinks(inputPath)
	if err != nil {
		// 路径可能不存在（如写入新文件），使用父目录解析
		parent := filepath.Dir(inputPath)
		resolvedParent, perr := filepath.EvalSymlinks(parent)
		if perr != nil {
			return nil, fmt.Errorf("cannot resolve path: %w", err)
		}
		resolvedInput = filepath.Join(resolvedParent, filepath.Base(inputPath))
	}

	// 使用解析后的路径进行安全验证
	fullPath := resolvedInput
	if !security.IsPathWithin(fullPath, t.workspace) {
		return nil, fmt.Errorf("path outside workspace: %s", params.Path)
	}

	logger.Debug("Filesystem action",
		logger.String("action", params.Action),
		logger.String("path", params.Path),
	)

	switch params.Action {
	case "read":
		return t.readFile(fullPath)
	case "write":
		return t.writeFile(fullPath, params.Content, params.Overwrite)
	case "list":
		return t.listDir(fullPath)
	case "delete":
		return t.deleteFile(fullPath)
	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}
}

func (t *FileSystemTool) readFile(path string) (json.RawMessage, error) {
	// 先检查是否可以去重
	offset := 1
	var limit *int = nil
	if t.fileStates != nil && t.fileStates.IsUnchanged(path, offset, limit) {
		result := map[string]any{
			"success": true,
			"path":    path,
			"content": "[unchanged since last read]",
			"cached":  true,
		}
		return json.Marshal(result)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// 记录读取操作
	if t.fileStates != nil {
		t.fileStates.RecordRead(path, offset, limit)
	}

	result := map[string]any{
		"success": true,
		"path":    path,
		"content": string(content),
	}
	return json.Marshal(result)
}

func (t *FileSystemTool) writeFile(path string, content string, overwrite bool) (json.RawMessage, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); err == nil && !overwrite {
		return nil, fmt.Errorf("file already exists: %s (use overwrite=true)", path)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	// 记录写入操作
	if t.fileStates != nil {
		t.fileStates.RecordWrite(path)
	}

	result := map[string]any{
		"success": true,
		"path":    path,
		"message": "File written successfully",
	}
	return json.Marshal(result)
}

func (t *FileSystemTool) listDir(path string) (json.RawMessage, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("list directory: %w", err)
	}

	files := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, map[string]any{
			"name":    e.Name(),
			"type":    func() string { if e.IsDir() { return "dir" } else { return "file" } }(),
			"size":    info.Size(),
			"modTime": info.ModTime(),
		})
	}

	result := map[string]any{
		"success": true,
		"path":    path,
		"entries": files,
	}
	return json.Marshal(result)
}

func (t *FileSystemTool) deleteFile(path string) (json.RawMessage, error) {
	// Soft-delete: move to .trash directory instead of permanent removal
	trashDir := filepath.Join(t.workspace, ".trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return nil, fmt.Errorf("create trash dir: %w", err)
	}

	// Generate unique name in trash to avoid collisions
	baseName := filepath.Base(path)
	destPath := filepath.Join(trashDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), baseName))

	if err := os.Rename(path, destPath); err != nil {
		return nil, fmt.Errorf("move to trash: %w", err)
	}

	result := map[string]any{
		"success":    true,
		"path":       path,
		"trash_path": destPath,
		"message":    "File moved to .trash directory. Permanent deletion requires explicit confirmation.",
	}
	return json.Marshal(result)
}
