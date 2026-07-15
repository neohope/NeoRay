# NeoRay 完整打包指南

本指南介绍如何构建和打包完整的 NeoRay 系统（后端 + 前端）。

## 项目结构

```
NeoRay/
├── neoray/              # Go 后端
│   ├── cmd/server/
│   ├── internal/
│   ├── scripts/
│   ├── Makefile
│   └── BUILD.md        # 后端打包文档
├── neoray_app/         # Flutter 前端
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
# 自动检测平台并构建（桌面）
make all

# 构建 Web 版本（后端 + 前端 Web）
make all-web

# 或指定平台
make windows
make macos
make linux
make web
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
make all          # 自动检测平台构建全部（桌面）
make all-web      # 构建全部（后端 + Web 前端）
make windows      # 构建 Windows 版本
make macos        # 构建 macOS 版本
make linux        # 构建 Linux 版本
make web          # 构建 Web 版本
make backend      # 仅构建后端
make frontend     # 仅构建前端（桌面）
make frontend-web # 仅构建前端（Web）
make clean        # 清理所有构建
```

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
config.yaml
config.yaml.example
```

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

## Web 版本部署

### 构建 Web 版本

```bash
# 方式 1：使用根目录 Makefile
make all-web

# 方式 2：使用前端脚本
cd neoray_app
scripts\build-web.bat  # Windows
# 或
scripts/build-web.sh   # macOS/Linux
```

### 运行 Web 版本

```bash
# 开发模式
cd neoray_app
make run-web

# 测试构建版本
cd neoray_app/build/web
python3 -m http.server 8080
# 浏览器打开 http://localhost:8080
```

### 部署到生产

将 `neoray_app/build/web` 目录部署到任何静态托管服务：
- Netlify
- Vercel
- GitHub Pages
- Nginx
- AWS S3
- 等等

**重要**：Web 版本需要后端 API 可访问，确保：
1. 后端服务已启动
2. 前端配置的 API 地址可访问（跨域配置）

## 相关文档

- [后端打包文档](neoray/BUILD.md)
- [前端打包文档](neoray_app/BUILD.md)
- [项目 README](README.md)
