package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	return &FileSystemTool{
		cfg:         cfg,
		workspace:   cfg.ResolvePath("workspace"),
		fileStates:  NewFileStates(),
	}
}

// NewFileSystemToolWithFileStates 创建带 FileStates 的文件系统工具
func NewFileSystemToolWithFileStates(cfg *config.Config, fileStates *FileStates) *FileSystemTool {
	return &FileSystemTool{
		cfg:         cfg,
		workspace:   cfg.ResolvePath("workspace"),
		fileStates:  fileStates,
	}
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
	fullPath := filepath.Join(t.workspace, params.Path)
	// 使用 IsPathWithin 进行更严格的验证
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
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("delete file: %w", err)
	}

	result := map[string]any{
		"success": true,
		"path":    path,
		"message": "File deleted successfully",
	}
	return json.Marshal(result)
}
