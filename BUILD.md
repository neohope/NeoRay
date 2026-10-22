# NeoRay 完整打包指南

本指南介绍如何构建和打包完整的 NeoRay 系统（后端 + 前端）。

## 项目结构

```
NeoRay/
├── neoray/              # Go 后端（含内嵌 Web UI 资源）
│   ├── cmd/server/
│   ├── internal/        # 含 internal/webui（go:embed 资源）
│   ├── scripts/         # 含 build-webui.sh
│   ├── Makefile
│   └── BUILD.md        # 后端打包文档
├── webui/              # Web UI 前端源码（React + Vite）
│   ├── src/
│   ├── public/
│   └── vite.config.ts
├── neoray_app/         # Flutter 客户端
│   ├── lib/
│   ├── scripts/
│   ├── Makefile
│   └── BUILD.md        # 前端打包文档
├── scripts/            # 完整构建脚本
│   ├── build-all-windows.bat
│   ├── build-all-macos.sh
│   └── build-all-linux.sh
├── Makefile           # 根目录 Makefile
└── BUILD.md           # 本文档
```

---

## 快速开始（推荐）

### 方式 1：一键构建全部（最简单）

使用根目录的一键构建脚本，自动构建后端 + 前端：

```bash
# Windows (桌面)
scripts\build-all-windows.bat

# macOS (桌面)
chmod +x scripts/build-all-macos.sh
./scripts/build-all-macos.sh

# Linux (桌面)
chmod +x scripts/build-all-linux.sh
./scripts/build-all-linux.sh
```

### 方式 2：使用根目录 Makefile

```bash
# 自动检测平台并构建（后端 + 桌面客户端，当前平台）
make all

# 仅构建后端（其构建脚本会先构建 Web UI 资源并内嵌）
make backend

# 仅构建 Web UI 前端资源（React + Vite，产物由后端 go:embed 使用）
make webui

# 仅构建桌面客户端（Flutter）
make frontend

# 或指定平台
make windows
make macos
make linux
```

---

## 完整构建流程（手动分步）

### 第一步：构建后端

```bash
cd neoray

# Windows
scripts\build-windows.bat

# macOS
chmod +x scripts/build-macos.sh
./scripts/build-macos.sh

# Linux
chmod +x scripts/build-linux.sh
./scripts/build-linux.sh
```

后端输出位置：
- Windows: `neoray/dist/neoray-windows.zip`
- macOS: `neoray/dist/neoray-macos.tar.gz`
- Linux: `neoray/dist/neoray-linux.tar.gz`

详细说明请参阅 [neoray/BUILD.md](neoray/BUILD.md)。

---

### 第二步：构建前端

```bash
cd neoray_app

# Windows
scripts\setup.bat          # 首次设置（仅一次）
scripts\build-windows.bat

# macOS
chmod +x scripts/build-macos.sh
./scripts/build-macos.sh

# Linux
chmod +x scripts/build-linux.sh
./scripts/build-linux.sh
```

前端输出位置：
- Windows: `neoray_app/dist/neoray-windows.zip`
- macOS: `neoray_app/dist/neoray-macos.dmg`
- Linux: `neoray_app/dist/neoray-linux.tar.gz`

详细说明请参阅 [neoray_app/BUILD.md](neoray_app/BUILD.md)。

---

## 根目录 Makefile 命令

```bash
make              # 显示帮助
make all          # 自动检测平台构建全部（后端 + 桌面客户端）
make backend      # 仅构建后端（含内嵌 Web UI）
make webui        # 仅构建 Web UI 前端资源（React + Vite）
make frontend     # 仅构建桌面客户端（Flutter）
make windows      # 构建 Windows 版本
make macos        # 构建 macOS 版本
make linux        # 构建 Linux 版本
make clean        # 清理所有构建
```

> `make web` 是 `make webui` 的别名（保持历史命令可用）。
> 注意：Web UI 的构建产物由后端 `go:embed` 内嵌进二进制，
> 因此 `backend` / `macos` / `windows` / `linux` 均已自动包含 Web UI 构建。

---

## Windows 快速指南

### 一键构建全部（推荐）

```bash
scripts\build-all-windows.bat
```

### 后端构建

```bash
cd neoray
scripts\build-windows.bat
```

### 前端构建

```bash
cd neoray_app
scripts\setup.bat          # 仅首次
scripts\build-windows.bat
```

### 运行

```bash
# 终端 1 - 启动后端
cd neoray\build
neoray.exe --no-tui

# 终端 2 - 启动前端
cd neoray_app
flutter run
```

---

## 发布包内容

### 后端包 (neoray-*.zip/tar.gz)

```
neoray.exe / neoray
neoray.toml.example
```

> 配置文件为 TOML 格式（`neoray.toml`），首次运行时会自动生成于 `~/.neoray/`。
> Web UI 资源已内嵌进二进制，发布包无需附带前端文件。

### 前端包 (neoray-*.zip/dmg/tar.gz)

```
neoray_app.exe / NeoRay.app / neoray_app
(平台特定文件)
```

---

## 部署建议

### 生产部署

1. **后端**：作为服务运行，使用 `--no-tui` 模式
2. **前端**：分发桌面应用安装包
3. **配置**：为用户提供配置向导

### 开发模式

```bash
# 终端 1 - 后端
cd neoray
make run

# 终端 2 - 前端
cd neoray_app
make run
```

---

## Web UI 部署

Web UI 资源由 `go:embed` 内嵌进后端二进制，**没有独立的 Web 构建产物需要部署**。

### 构建

```bash
# 1. 构建前端资源（输出到 neoray/internal/webui/dist，供 go:embed 打包）
cd webui
npm install
npm run build

# 2. 编译后端（自动内嵌上一步产物）
cd ../neoray
go build -o neoray ./cmd/server
```

后端构建脚本已接入前端构建（`neoray/scripts/build-webui.sh`）。
未安装 Node 时会自动跳过，此时服务端显示内置提示页，不影响其他功能。

### 启用与访问

在 `~/.neoray/neoray.toml` 中启用：

```toml
[web]
enabled = true
```

启动后浏览器访问 `http://127.0.0.1:8080`。

首次以 `web.enabled = true` 启动且 `security.auth.secret_key` 为空时，
程序会自动生成强随机密钥、写入配置文件并打印一次，登录时输入该密钥。

### 安全说明

- Web UI 默认**关闭**；关闭时行为与未引入该功能时完全一致。
- 服务默认仅监听 `127.0.0.1`。若绑定到非回环地址，启动时会输出告警。
- Web UI 使用 Cookie 会话鉴权（浏览器 WebSocket 无法自定义请求头）；
  CLI / Flutter 客户端仍使用 `X-Api-Key` / `Bearer` 请求头，两者互不影响。

### 开发模式

改前端时无需重新编译 Go 二进制，可在配置中指向磁盘目录：

```toml
[web]
enabled = true
static_dir = "../neoray/internal/webui/dist"
```

或使用 Vite dev server（已内置代理到 `127.0.0.1:8080`）：

```bash
cd webui && npm run dev
```

## 相关文档

- [后端打包文档](neoray/BUILD.md)
- [前端打包文档](neoray_app/BUILD.md)
- [项目 README](README.md)

---

## 代码审计

五轮代码审计（问题、修复提交、验证方式、仍保留的决策项）见 [docs/audit/README.md](docs/audit/README.md)。
