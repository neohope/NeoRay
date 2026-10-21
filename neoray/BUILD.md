# NeoRay 后端打包指南

本指南介绍如何构建和打包 NeoRay 后端服务。

## 前置要求

- Go 1.21 或更高版本
- 对应平台的编译工具链

### 安装 Go

访问 https://golang.org/dl/ 下载并安装 Go。

验证安装：
```bash
go version
```

---

## 快速开始（Windows）

### 方式 1：使用构建脚本（推荐）

```bash
cd neoray
scripts\build-windows.bat
```

### 方式 2：使用 Makefile

```bash
cd neoray
make windows
```

### 方式 3：手动构建

```bash
cd neoray
go mod tidy
go build -ldflags "-s -w" -o neoray.exe ./cmd/server
```

---

## 平台构建命令

### Windows

```bash
# 脚本方式
scripts\build-windows.bat

# 或 Makefile
make windows

# 或手动
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o build/neoray.exe ./cmd/server
```

### macOS

```bash
# 脚本方式
chmod +x scripts/build-macos.sh
./scripts/build-macos.sh

# 或 Makefile
make macos

# 或手动
export GOOS=darwin
export GOARCH=arm64  # Apple Silicon
# 或 export GOARCH=amd64  # Intel
go build -ldflags "-s -w" -o build/neoray ./cmd/server
```

### Linux

```bash
# 脚本方式
chmod +x scripts/build-linux.sh
./scripts/build-linux.sh

# 或 Makefile
make linux

# 或手动
export GOOS=linux
export GOARCH=amd64
go build -ldflags "-s -w" -o build/neoray ./cmd/server
```

---

## 输出位置

构建完成后，文件位于：

| 平台 | 输出文件 |
|-------|---------|
| **Windows** | `dist/neoray-windows.zip` |
| **macOS** | `dist/neoray-macos.tar.gz` |
| **Linux** | `dist/neoray-linux.tar.gz` |

可执行文件位于 `build/` 目录。

---

## 运行方式

### 完整模式（TUI + API）

```bash
# Windows
neoray.exe

# macOS/Linux
./neoray
```

### 仅服务器模式（无 TUI）

```bash
# Windows
neoray.exe --no-tui

# macOS/Linux
./neoray --no-tui
```

### 指定配置文件

```bash
neoray --config /path/to/neoray.toml
```

> 配置文件为 TOML 格式，默认名 `neoray.toml`（由 `SetConfigName("neoray")` 决定）。
> 未显式指定时按以下顺序查找：`~/.neoray/` → 当前目录 → 可执行文件同目录。
> 首次运行时会在 `~/.neoray/` 自动生成一份默认配置。

---

## Makefile 命令参考

```bash
make              # 显示帮助
make all          # 构建所有平台
make windows      # 构建 Windows
make macos        # 构建 macOS
make linux        # 构建 Linux
make test         # 运行测试
make clean        # 清理
make run          # 运行
make install      # 安装到 ~/.local/bin 并同步内置技能
make install-skills  # 只同步内置技能到 ~/.neoray/skills
make fmt          # 格式化代码
```

### 安装做了什么

`make install` 包含两步（`install-skills` 是它的前置依赖）：

1. **同步内置技能** `skills/*` → `~/.neoray/skills/`
   服务端按 `config.HomeDir/skills` 加载技能；不拷贝的话运行时会加载到
   **0 个技能**，agent 会答不出"我有哪些技能"。
2. **构建并安装二进制** → `$(BIN_DIR)/neoray`（默认 `~/.local/bin`）

> 特意不用 `go install`：它会把二进制放到 `GOPATH/bin`，与 launchd 服务
> 实际启动的路径不是同一个地方，升级后容易出现"装了但没生效"。
> 自定义位置可用 `make install BIN_DIR=/usr/local/bin`，
> 技能目录可用 `NEORAY_SKILLS_DIR=/path/to/skills` 覆盖。

安装后需重启服务生效：

```bash
launchctl kickstart -k gui/$(id -u)/ai.neoray.gateway
```

---

## 配置文件

构建脚本会复制配置样例：

- `neoray.toml.example` → `build/neoray.toml.example`

**重要**：实际使用的配置文件为 `~/.neoray/neoray.toml`（TOML 格式），
首次运行时会自动生成。如需自定义，请参照 `neoray.toml.example` 编辑。

> Web UI 相关资源已由 `go:embed` 内嵌进二进制，发布包无需额外附带前端文件。

---

## 交叉编译

可以在一个平台上为其他平台构建：

### 在 Windows 上构建 Linux/macOS

```bash
# Linux
set GOOS=linux
set GOARCH=amd64
go build -o neoray ./cmd/server

# macOS
set GOOS=darwin
set GOARCH=arm64
go build -o neoray ./cmd/server
```

### 在 macOS 上构建 Windows/Linux

```bash
# Windows
export GOOS=windows
export GOARCH=amd64
go build -o neoray.exe ./cmd/server

# Linux
export GOOS=linux
export GOARCH=amd64
go build -o neoray ./cmd/server
```

---

## 构建优化

使用 `-ldflags "-s -w"` 减小二进制文件大小：

- `-s`：移除符号表
- `-w`：移除 DWARF 调试信息

可以减小约 30-50% 的文件大小。

---

## 目录结构

```
neoray/
├── cmd/
│   └── server/
│       └── main.go          # 主程序入口
├── internal/                # 内部包
├── scripts/
│   ├── build-windows.bat   # Windows 构建脚本
│   ├── build-macos.sh      # macOS 构建脚本
│   └── build-linux.sh      # Linux 构建脚本
├── Makefile                # Makefile
├── BUILD.md               # 本文档
└── neoray.toml.example    # 配置样例（实际配置生成于 ~/.neoray/neoray.toml）
```

---

## 故障排除

### 问题：`go: command not found`

**解决**：确保 Go 已安装并添加到 PATH。

### 问题：依赖下载失败

**解决**：
```bash
go env -w GOPROXY=https://goproxy.cn,direct
go mod tidy
```

### 问题：测试失败但仍想构建

**解决**：构建脚本会继续执行，测试失败不影响构建。

---

## 下一步

后端打包完成后，请参阅 [neoray_app/BUILD.md](../neoray_app/BUILD.md) 进行前端打包。
