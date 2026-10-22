
# NeoRay

AI Agent 项目，基于 Go（后端）构建，提供 Flutter 跨平台客户端与内嵌 Web UI。

## 项目概述

NeoRay 是一个轻量级、异步的 AI 代理框架，专为构建企业级聊天机器人和智能助手而设计。项目灵感来源于 neobot，采用模块化、插件式架构，支持多种聊天平台集成。

## 技术栈

- **后端**: Go（标准库 net/http + gorilla/websocket）
- **桌面/移动客户端**: Flutter + Riverpod
- **Web UI**: React + TypeScript + Vite（产物经 go:embed 内嵌进后端二进制）
- **LLM 支持**: Anthropic Claude, OpenAI 兼容 API

## 功能特性

- 核心代理引擎 - 状态机驱动的消息处理
- 消息总线系统 - 异步队列和事件发布/订阅
- 多平台频道集成 - WebSocket, 飞书/Lark
- 丰富的工具系统 - 文件操作, Shell, Web 搜索
- 会话和配置管理 - 持久化, 强类型配置
- 安全模块 - 工作区限制, 沙箱执行
- **内嵌 Web UI** - 单个二进制即可通过浏览器访问，无需额外安装

## 文档

- [架构设计](docs/ARCHITECTURE.md) - 整体架构和功能模块分析
- [后端配置](docs/BACKEND_CONFIG.md) - 配置项说明
- [前端架构](docs/FRONTEND_ARCHITECTURE.md) - Flutter 客户端结构
- [实现路线图](docs/IMPLEMENTATION_PLAN.md) - 分阶段实现计划
- [项目结构说明](docs/项目结构说明.md) - 目录职责与重构建议

## 项目结构

```
NeoRay/
├── neoray/        # Go 后端（含内嵌 Web UI 资源）
├── webui/         # Web UI 前端源码（React + Vite）
├── neoray_app/    # Flutter 客户端
├── scripts/       # 跨平台构建编排
├── design/        # 设计文件
└── docs/          # 文档
```

### 用户目录 (运行时)
配置、日志、数据默认存储在用户目录：
```
# Windows: C:\Users\YourName\.neoray\
# macOS/Linux: ~/.neoray/
├── neoray.toml   # 主配置文件
├── config.toml.example  # 完整配置项参考样例（不生效）
├── logs/         # 日志
├── data/         # 数据库
└── workspace/    # 工具工作区
```

> 配置文件名为 `neoray.toml`（由 `config.Load` 的 `SetConfigName("neoray")` 决定）。
> `config.toml.example` 仅作参考，程序不会读取。

## 快速开始

### 后端

```bash
cd neoray
go build -o neoray ./cmd/server
./neoray
```

### Web UI

Web UI 资源会被内嵌进后端二进制，启动后浏览器访问 `http://127.0.0.1:8080`：

```bash
# 1. 构建前端资源（可选：不构建时服务端显示内置提示页）
cd webui
npm install
npm run build

# 2. 编译后端（自动内嵌上一步产物）
cd ../neoray
go build -o neoray ./cmd/server

# 3. 在 neoray.toml 中启用
# [web]
# enabled = true

# 4. 启动
./neoray
```

首次以 `web.enabled = true` 启动且未配置 `security.auth.secret_key` 时，
程序会自动生成强随机密钥、写入配置文件并打印一次，登录时输入该密钥即可。

### 构建脚本

```bash
# 后端（含 Web UI 资源）
cd neoray && ./scripts/build-macos.sh
```

## 功能状态

- ✅ 后端核心：代理引擎、消息总线、工具系统、会话与配置管理
- ✅ 多渠道：飞书 / WebSocket
- ✅ 内嵌 Web UI：静态托管、Cookie 鉴权、流式对话
- ✅ 记忆系统：Git 存储、Dream / Consolidator / AutoCompact
- ✅ 定时任务、子代理系统
- 🚧 测试覆盖薄弱（核心包缺少单测）

## License

BSD 3-Clause License - see [LICENSE](LICENSE) file for details

---

## 代码审计

四轮代码审计（问题、修复提交、验证方式、仍保留的决策项）见 [docs/audit/README.md](docs/audit/README.md)。
