
# NeoRay 项目架构文档

## 项目概述

**NeoRay** 是一个用 Go（后端）和 Flutter（前端）构建的 AI Agent 项目，灵感来源于 neobot。它提供一个轻量级、异步的 AI 代理框架，用于构建企业级聊天机器人和智能助手。

---

## 从 neobot 继承的核心功能模块

### 1. 核心代理引擎 (Core Agent Engine)
- **状态机驱动的消息处理流程**
- **会话管理和上下文构建**
- **工具执行协调**
- **多轮对话管理**
- **错误处理和重试机制**

### 2. 消息总线系统 (Message Bus)
- **异步消息队列**
- **入站/出站消息分离**
- **事件发布/订阅模型**

### 3. 频道集成模块 (Channel Integration)
- **统一的频道接口**
- **飞书/Lark 平台集成**
- **WebSocket 支持**
- **富文本消息和卡片渲染**
- **媒体文件处理**

### 4. 工具系统 (Tool System)
- **工具注册表**
- **动态工具加载**
- **文件系统工具**
- **Shell 执行工具**
- **Web 搜索工具**
- **定时任务工具**

### 5. LLM 提供商系统 (LLM Provider System)
- **统一的 LLM 接口**
- **Anthropic Claude 支持**
- **OpenAI 兼容 API 支持**
- **可扩展的提供商架构**

### 6. 会话和配置系统 (Session &amp; Config)
- **会话管理和持久化**
- **强类型配置**
- **环境变量支持**
- **记忆和内容整合**

### 7. 安全模块 (Security)
- **工作区访问控制**
- **网络策略**
- **沙箱执行**

---

## NeoRay 技术栈

### 后端 (Go)
- **Web 框架**: Gin 或 Fiber
- **异步处理**: Goroutines + Channels
- **配置**: Viper
- **日志**: Zap 或 Logrus
- **数据库**: SQLite (开发) / PostgreSQL (生产)
- **WebSocket**: gorilla/websocket

### 前端 (Flutter)
- **目标平台**: Windows, macOS, Linux, Web
- **状态管理**: Riverpod
- **UI**: Material 3 (桌面优化)
- **窗口管理**: window_manager (桌面端)
- **多窗口支持**: desktop_multi_window (可选)
- **WebSocket**: web_socket_channel
- **Markdown 渲染**: flutter_markdown
- **代码高亮**: flutter_highlight
- **文件选择器**: file_picker (桌面端)
- **系统托盘**: system_tray (桌面端)

---

## 目录结构规划

### 项目目录 (开发时)
```
NeoRay/
├── neoray/              # Go 后端 (项目名: neoray)
│   ├── cmd/             # 入口程序
│   │   └── server/
│   ├── internal/        # 内部模块
│   │   ├── agent/       # 核心代理引擎
│   │   ├── bus/         # 消息总线
│   │   ├── channel/     # 频道集成
│   │   ├── config/      # 配置
│   │   ├── provider/    # LLM 提供商
│   │   ├── session/     # 会话管理
│   │   ├── security/    # 安全模块
│   │   └── tools/       # 工具系统
│   └── pkg/             # 公共包
│
├── neorayui/            # Flutter 前端 (项目名: neorayui)
│   ├── lib/
│   │   ├── features/    # 功能模块
│   │   ├── providers/   # 状态管理
│   │   ├── services/    # API 服务
│   │   └── widgets/     # 组件
│
├── config/              # 默认配置模板
│   └── config.toml      # 默认配置
│
└── docs/                # 文档
```

### 用户目录 (运行时)
配置、日志、数据默认存储在 `~/.neoray/` 中：
```
# Windows
C:\Users\YourName\.neoray\
├── config.toml         # 配置文件
├── logs/               # 日志
├── data/               # 数据库
└── workspace/          # 工具工作区

# macOS / Linux
~/.neoray/
├── config.toml
├── logs/
├── data/
└── workspace/
```

---

## 实现步骤（分阶段）

### Phase 1: 基础框架 (Week 1-2)
- [ ] 项目初始化和目录结构搭建
- [ ] 配置系统实现
- [ ] 日志系统实现
- [ ] 基础消息总线

### Phase 2: 核心代理引擎 (Week 3-4)
- [ ] 会话管理
- [ ] 上下文构建器
- [ ] Agent 主循环
- [ ] LLM 提供商接口

### Phase 3: 工具系统 (Week 5-6)
- [ ] 工具注册表
- [ ] 文件系统工具
- [ ] Shell 工具
- [ ] Web 工具

### Phase 4: 频道集成 (Week 7-8)
- [ ] WebSocket 频道
- [ ] 飞书频道
- [ ] 消息格式转换

### Phase 5: 前端开发 (Week 9-10)
- [ ] Flutter 项目初始化
- [ ] 聊天界面
- [ ] WebSocket 连接
- [ ] 会话管理 UI

### Phase 6: 安全和优化 (Week 11-12)
- [ ] 安全模块
- [ ] 性能优化
- [ ] 测试和文档

---

## 核心设计原则

1. **模块化**: 各组件松散耦合，通过消息总线交互
2. **可扩展**: 插件式架构，易于添加新工具和频道
3. **异步**: 全异步设计，支持高并发
4. **安全**: 工作区限制、参数验证、沙箱执行
5. **类型安全**: Go 的强类型 + Flutter 的类型系统

