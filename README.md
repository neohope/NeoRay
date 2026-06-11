
# NeoRay

AI Agent 项目，基于 Go（后端）和 Flutter（前端）构建。

## 项目概述

NeoRay 是一个轻量级、异步的 AI 代理框架，专为构建企业级聊天机器人和智能助手而设计。项目灵感来源于 neobot，采用模块化、插件式架构，支持多种聊天平台集成。

## 技术栈

- **后端**: Go + Gin/Fiber
- **前端**: Flutter + Riverpod/Bloc
- **LLM 支持**: Anthropic Claude, OpenAI 兼容 API

## 功能特性

- 核心代理引擎 - 状态机驱动的消息处理
- 消息总线系统 - 异步队列和事件发布/订阅
- 多平台频道集成 - WebSocket, 飞书/Lark
- 丰富的工具系统 - 文件操作, Shell, Web 搜索
- 会话和配置管理 - 持久化, 强类型配置
- 安全模块 - 工作区限制, 沙箱执行

## 文档

- [架构设计](docs/ARCHITECTURE.md) - 整体架构和功能模块分析
- [实现路线图](docs/IMPLEMENTATION_PLAN.md) - 分阶段实现计划

## 项目结构

### 项目目录 (开发时)
```
NeoRay/
├── neoray/        # Go 后端 (项目名: neoray)
├── neorayui/      # Flutter 前端 (项目名: neorayui)
├── config/        # 配置模板
└── docs/          # 文档
```

### 用户目录 (运行时)
配置、日志、数据默认存储在用户目录：
```
# Windows: C:\Users\YourName\.neoray\
# macOS/Linux: ~/.neoray/
├── config.toml   # 配置文件
├── logs/         # 日志
├── data/         # 数据库
└── workspace/    # 工具工作区
```

## 快速开始

### 后端

```bash
cd neoray
go build -o neoray.exe ./cmd/server
./neoray.exe
```

### 功能状态

✅ **Phase 1: 项目初始化** - 完成
- 项目结构和配置
- BSD 3-Clause 许可证
- TOML 配置管理
- 日志系统
- 用户目录 `~/.neoray/` 自动创建

✅ **Phase 2: TUI 聊天界面** - 进行中
- Bubble Tea TUI 框架
- 会话管理 (内存存储)
- LLM Provider 抽象
- 支持 Anthropic Claude 和 OpenAI

🚧 **Phase 3-6**: 待实现

## 开发状态

🚀 Phase 1 完成 | Phase 2 进行中

## License

BSD 3-Clause License - see [LICENSE](LICENSE) file for details

