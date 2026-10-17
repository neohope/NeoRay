package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"neoray/internal/logger"
	"neoray/internal/webui"
)

// staticHandler 提供 Web UI 静态资源服务。
//
// 资源来源有两种模式（优先级从高到低）：
//  1. 磁盘模式：config.Web.StaticDir 非空且目录存在时，从磁盘读取。
//     用于开发调试——改前端无需重新编译 Go 二进制。
//  2. 内嵌模式：从 go:embed 打包进二进制的资源读取。
//     用于正式发布——用户只需一个可执行文件，无需额外安装。
//
// 未匹配到静态文件时回退到 index.html，以支持前端 SPA 客户端路由。
type staticHandler struct {
	// diskDir 非空时使用磁盘资源
	diskDir string
	// embedded 内嵌资源文件系统（根即站点根）
	embedded fs.FS
	// indexHTML 预读的 index.html 内容（SPA 回退用）
	indexHTML []byte
	// placeholder 为 true 表示前端尚未构建，当前提供的是内置提示页
	placeholder bool
}

// newStaticHandler 创建静态资源处理器。
// 返回 nil 表示既无可用磁盘目录、也无内嵌资源，调用方应跳过注册。
func newStaticHandler(cfgWebStaticDir string) *staticHandler {
	h := &staticHandler{}

	// 优先磁盘模式
	if cfgWebStaticDir != "" {
		if info, err := os.Stat(cfgWebStaticDir); err == nil && info.IsDir() {
			h.diskDir = cfgWebStaticDir
			logger.Info("Web UI serving from disk (dev mode)",
				logger.String("dir", cfgWebStaticDir))
		} else {
			logger.Warn("Web UI static_dir configured but not usable, falling back to embedded",
				logger.String("dir", cfgWebStaticDir))
		}
	}

	// 内嵌模式
	if h.diskDir == "" {
		fsys, err := webui.FS()
		if err != nil {
			logger.Error("Failed to open embedded Web UI", logger.ErrorField(err))
			return nil
		}
		h.embedded = fsys
		logger.Info("Web UI serving from embedded assets")
	}

	// 读取 index.html。若尚不存在（源码仓库未构建前端），
	// 使用内置提示页作为回退 —— 这样 go build 与运行时都不会失败，
	// 且无需在版本库中跟踪任何构建产物。
	idx, err := h.readFile("index.html")
	if err != nil {
		logger.Warn("Web UI assets not built yet; serving built-in placeholder page. " +
			"Run: cd webui && npm install && npm run build")
		h.indexHTML = []byte(placeholderPage)
		h.placeholder = true
	} else {
		h.indexHTML = idx
	}

	return h
}

// placeholderPage 是前端未构建时提供的最小提示页（内嵌于 Go 代码，不入库静态文件）。
const placeholderPage = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>NeoRay Web UI</title>
  <style>
    body { margin:0; font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;
      background:#f7f8fa; color:#1f2328; display:flex; align-items:center;
      justify-content:center; min-height:100vh; }
    .box { max-width:560px; padding:40px; text-align:center; line-height:1.8; }
    h1 { font-size:22px; margin-bottom:12px; }
    code { background:#f1f3f5; padding:2px 8px; border-radius:4px;
      font-family:"SF Mono",Menlo,monospace; font-size:13px; color:#c026a3; }
    .hint { color:#6b7280; font-size:14px; margin-top:20px; }
  </style>
</head>
<body>
  <div class="box">
    <h1>🐰 NeoRay Web UI 尚未构建</h1>
    <p>服务端已启用 Web UI，但前端资源尚未生成。请执行：</p>
    <p><code>cd webui && npm install && npm run build</code></p>
    <p class="hint">构建产物会自动嵌入 Go 二进制，之后重新编译并重启 neoray 即可访问。</p>
  </div>
</body>
</html>`

// readFile 按当前模式读取指定相对路径的文件。
func (h *staticHandler) readFile(name string) ([]byte, error) {
	if h.diskDir != "" {
		// 防目录穿越：清理后拼接，并校验结果仍在 diskDir 之内
		clean := filepath.Clean("/" + name)
		full := filepath.Join(h.diskDir, clean)
		return os.ReadFile(full)
	}
	return fs.ReadFile(h.embedded, name)
}

// openFile 打开指定相对路径的文件（用于 http.FileServer 语义或流式读取）。
func (h *staticHandler) openFile(name string) (fs.File, error) {
	if h.diskDir != "" {
		clean := filepath.Clean("/" + name)
		full := filepath.Join(h.diskDir, clean)
		return os.Open(full)
	}
	return h.embedded.Open(name)
}

// stat 返回文件信息。
func (h *staticHandler) stat(name string) (fs.FileInfo, error) {
	if h.diskDir != "" {
		clean := filepath.Clean("/" + name)
		full := filepath.Join(h.diskDir, clean)
		return os.Stat(full)
	}
	return fs.Stat(h.embedded, name)
}

// setSecurityHeaders 设置静态资源响应头。
// CSP 允许内联样式（Vite 构建产物可能内联）但禁止内联脚本，以降低 XSS 风险。
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self'; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data: blob:; "+
			"font-src 'self' data:; "+
			"connect-src 'self' ws: wss:; "+
			"object-src 'none'; "+
			"base-uri 'self'; "+
			"frame-ancestors 'none'")
}

// ServeHTTP 实现 http.Handler。
func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 仅允许 GET / HEAD
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	setSecurityHeaders(w)

	// 规范化路径：去掉前导 /，空路径视为 index.html
	rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}

	// 尝试命中静态文件
	if info, err := h.stat(rel); err == nil && !info.IsDir() {
		h.serveFile(w, r, rel, info)
		return
	}

	// 未命中：SPA 回退到 index.html（客户端路由）
	// 带扩展名且非 .html 的请求视为资源缺失，返回 404 而非回退
	if ext := path.Ext(rel); ext != "" && ext != ".html" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// index.html 不缓存，保证前端更新后立即生效
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(h.indexHTML)
}

// handleAPINotFound 处理未注册的 /api/ 路径，返回 JSON 404。
// 存在意义：避免未注册的 API 路径被 "/" 静态路由的 SPA 回退吞掉，
// 导致调用方拿到 HTML 而非 JSON。
func (s *Server) handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"API endpoint not found","path":` +
		jsonString(r.URL.Path) + `}`))
}

// jsonString 将字符串安全地序列化为 JSON 字符串字面量（含引号）。
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// warnIfWebExposed 在 Web UI 启用但监听地址非回环时告警。
// Web UI 面向本机浏览器使用，若暴露到非回环地址（0.0.0.0 / 公网 IP），
// 将把管理界面暴露给同网络的其他主机，存在安全风险。
func (s *Server) warnIfWebExposed() {
	host := s.cfg.Server.Host
	if host == "" || host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return
	}
	logger.Warn("Web UI is enabled but server is NOT bound to loopback — "+
		"the management interface may be reachable from other hosts",
		logger.String("server.host", host),
		logger.String("server.port", fmt.Sprintf("%d", s.cfg.Server.Port)),
	)
}

// serveFile 输出单个静态文件，按类型设置缓存策略。
func (h *staticHandler) serveFile(w http.ResponseWriter, r *http.Request, name string, info fs.FileInfo) {
	// Vite 产物带内容哈希，可长期强缓存；其他资源用协商缓存
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	f, err := h.openFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	seeker, ok := f.(interface {
		Read([]byte) (int, error)
		Seek(int64, int) (int64, error)
	})
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, name, info.ModTime(), seeker)
}
