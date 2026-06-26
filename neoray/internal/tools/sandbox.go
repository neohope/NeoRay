package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SandboxBackend 定义了沙盒后端接口
type SandboxBackend interface {
	// WrapCommand 将命令包装在沙盒中
	WrapCommand(command string, workspace string, cwd string) (string, error)
}

// BwrapBackend 实现 bubblewrap 沙盒后端
type BwrapBackend struct {
	mediaDir string
}

// NewBwrapBackend 创建 bubblewrap 沙盒后端
func NewBwrapBackend(mediaDir string) *BwrapBackend {
	return &BwrapBackend{
		mediaDir: mediaDir,
	}
}

// WrapCommand 实现 SandboxBackend 接口
func (b *BwrapBackend) WrapCommand(command string, workspace string, cwd string) (string, error) {
	ws, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	ws = filepath.Clean(ws)

	// 确定沙盒中的工作目录
	sandboxCwd := ws
	if cwd != "" {
		absCwd, err := filepath.Abs(cwd)
		if err == nil {
			absCwd = filepath.Clean(absCwd)
			// 检查是否在工作区内部
			if strings.HasPrefix(absCwd, ws+string(os.PathSeparator)) || absCwd == ws {
				sandboxCwd = absCwd
			}
		}
	}

	// 构建 bwrap 参数
	args := []string{"bwrap", "--new-session", "--die-with-parent"}

	// 绑定必要的系统目录（只读）
	requiredDirs := []string{"/usr"}
	for _, dir := range requiredDirs {
		if _, err := os.Stat(dir); err == nil {
			args = append(args, "--ro-bind", dir, dir)
		}
	}

	// 可选的系统目录（只读）
	optionalDirs := []string{
		"/bin", "/lib", "/lib64",
		"/etc/alternatives", "/etc/ssl/certs",
		"/etc/resolv.conf", "/etc/ld.so.cache",
	}
	for _, dir := range optionalDirs {
		if _, err := os.Stat(dir); err == nil {
			args = append(args, "--ro-bind-try", dir, dir)
		}
	}

	// 挂载 /proc 和 /dev
	args = append(args, "--proc", "/proc", "--dev", "/dev")

	// 创建临时目录 /tmp
	args = append(args, "--tmpfs", "/tmp")

	// 屏蔽工作区的父目录（防止访问配置文件）
	wsParent := filepath.Dir(ws)
	args = append(args, "--tmpfs", wsParent)

	// 重新创建工作区目录并挂载
	args = append(args, "--dir", ws)
	args = append(args, "--bind", ws, ws)

	// 挂载媒体目录（只读，如果存在）
	if b.mediaDir != "" {
		mediaAbs, err := filepath.Abs(b.mediaDir)
		if err == nil {
			mediaAbs = filepath.Clean(mediaAbs)
			if _, err := os.Stat(mediaAbs); err == nil {
				args = append(args, "--ro-bind-try", mediaAbs, mediaAbs)
			}
		}
	}

	// 设置工作目录
	args = append(args, "--chdir", sandboxCwd)

	// 添加要执行的命令
	args = append(args, "--", "sh", "-c", command)

	// 拼接成完整的命令字符串
	return escapeShellArgs(args), nil
}

// escapeShellArgs 转义 shell 参数
func escapeShellArgs(args []string) string {
	var builder strings.Builder
	for i, arg := range args {
		if i > 0 {
			builder.WriteByte(' ')
		}
		if arg == "" {
			builder.WriteString(`''`)
			continue
		}
		// 检查是否包含特殊字符
		if strings.ContainsAny(arg, " \t\n\r\f\v\"'\\`$*?[](){}|&;!<>~#=") {
			// 使用单引号包裹，并将内部的单引号替换为 '\''
			builder.WriteByte('\'')
			builder.WriteString(strings.ReplaceAll(arg, "'", `'\''`))
			builder.WriteByte('\'')
		} else {
			builder.WriteString(arg)
		}
	}
	return builder.String()
}

// SandboxRegistry 管理可用的沙盒后端
type SandboxRegistry struct {
	backends map[string]SandboxBackend
}

// NewSandboxRegistry 创建沙盒注册表
func NewSandboxRegistry(mediaDir string) *SandboxRegistry {
	return &SandboxRegistry{
		backends: map[string]SandboxBackend{
			"bwrap": NewBwrapBackend(mediaDir),
		},
	}
}

// WrapCommand 使用指定的沙盒后端包装命令
func (r *SandboxRegistry) WrapCommand(sandbox string, command string, workspace string, cwd string) (string, error) {
	if sandbox == "" {
		return command, nil
	}

	if runtime.GOOS == "windows" {
		// Windows 不支持沙盒
		return command, nil
	}

	backend, ok := r.backends[sandbox]
	if !ok {
		return "", fmt.Errorf("unknown sandbox backend: %s", sandbox)
	}

	return backend.WrapCommand(command, workspace, cwd)
}

// AvailableBackends 返回可用的沙盒后端列表
func (r *SandboxRegistry) AvailableBackends() []string {
	backends := make([]string, 0, len(r.backends))
	for name := range r.backends {
		backends = append(backends, name)
	}
	return backends
}

// globalSandboxRegistry 全局沙盒注册表（懒加载）
var globalSandboxRegistry *SandboxRegistry

// GetSandboxRegistry 获取全局沙盒注册表
func GetSandboxRegistry(mediaDir string) *SandboxRegistry {
	if globalSandboxRegistry == nil {
		globalSandboxRegistry = NewSandboxRegistry(mediaDir)
	}
	return globalSandboxRegistry
}
