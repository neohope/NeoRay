// Package webui 提供内嵌的 Web UI 静态资源。
//
// 前端（React + Vite）构建产物输出到本包的 dist/ 目录，并被编译进 Go 二进制，
// 使用户侧无需安装 Node 即可通过浏览器访问 Web UI。
//
// 关于 dist/ 目录：
//   - dist/* 为构建产物，不纳入版本控制（见 .gitignore）。
//   - dist/.gitkeep 必须保留在版本库中：go:embed 要求目标目录存在且非空，
//     否则全新克隆后 go build 会直接失败。
//   - 未构建前端时，服务端会回退到内置提示页（见 api/static.go），
//     因此运行与编译均不会中断。
package webui

import (
	"embed"
	"io/fs"
)

// distFS 内嵌构建产物。
// 使用 all: 前缀以确保包含以 "_" 或 "." 开头的文件（Vite 资源可能命中）。
//
//go:embed all:dist
var distFS embed.FS

// FS 返回内嵌静态资源文件系统（已剥离 dist/ 前缀，根即为站点根目录）。
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
