package tools

import (
	"os/exec"
	"runtime"
	"syscall"
)

// killProcessGroup 结束命令及其拉起的全部子进程。
// 只对主进程发信号会让 `sh -c "server &"` 里的子进程变成孤儿继续运行。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
		return
	}

	// 子进程以自身为组长启动（见 spawnCommand 的 Setpgid），
	// 因此可以按 -pid 杀掉整个进程组。
	pid := cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}

// setNewProcessGroup 让命令成为新进程组的组长（Unix）。
// 这样 killProcessGroup 的 kill(-pid) 才能覆盖它的后代进程。
func setNewProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || runtime.GOOS == "windows" {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}
