
# NeoRay 实现路线图

## 概述

本文档详细描述 NeoRay 项目的实现步骤，按阶段分解任务。

---

## Phase 1: 项目初始化和基础框架 (Week 1-2)

### 1.1 项目结构搭建
```
neoray/                   # Go 后端 (项目名: neoray)
├── go.mod
├── go.sum
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   └── logger/
└── pkg/
```

### 1.2 配置系统
- **文件**: `internal/config/config.go`
- **功能**:
  - 配置文件加载 (YAML/TOML)
  - 环境变量支持
  - 强类型配置结构
  - 默认值设置

### 1.3 日志系统
- **文件**: `internal/logger/logger.go`
- **功能**:
  - 结构化日志
  - 多级别日志
  - 文件和控制台输出

### 1.4 基础消息总线
- **文件**: `internal/bus/bus.go`
- **功能**:
  - 异步消息队列
  - 发布/订阅模式
  - 消息类型定义

---

## Phase 2: 核心代理引擎 (Week 3-4)

### 2.1 会话管理
- **文件**: `internal/session/session.go`
- **功能**:
  - 会话创建和销毁
  - 会话持久化
  - 会话状态管理

### 2.2 上下文构建器
- **文件**: `internal/agent/context.go`
- **功能**:
  - 历史消息管理
  - 系统提示注入
  - 上下文压缩

### 2.3 Agent 主循环
- **文件**: `internal/agent/loop.go`
- **功能**:
  - 状态机驱动
  - 消息处理流程
  - 工具调用协调

### 2.4 LLM 提供商接口
- **文件**: `internal/provider/provider.go`
- **功能**:
  - 统一接口定义
  - Anthropic Claude 实现
  - OpenAI 兼容实现

---

## Phase 3: 工具系统 (Week 5-6)

### 3.1 工具注册表
- **文件**: `internal/tools/registry.go`
- **功能**:
  - 工具注册
  - 参数验证
  - 执行调度

### 3.2 文件系统工具
- **文件**: `internal/tools/filesystem.go`
- **功能**:
  - 读取文件
  - 写入文件
  - 列出目录
  - 编辑文件

### 3.3 Shell 工具
- **文件**: `internal/tools/shell.go`
- **功能**:
  - 命令执行
  - 安全限制
  - 输出捕获

### 3.4 Web 工具
- **文件**: `internal/tools/web.go`
- **功能**:
  - Web 搜索
  - 内容抓取

---

## Phase 4: 频道集成 (Week 7-8)

### 4.1 基础频道接口
- **文件**: `internal/channel/base.go`
- **功能**:
  - 统一接口定义
  - 消息格式转换

### 4.2 WebSocket 频道
- **文件**: `internal/channel/websocket.go`
- **功能**:
  - WebSocket 服务
  - 消息收发
  - 连接管理

### 4.3 飞书频道
- **文件**: `internal/channel/feishu.go`
- **功能**:
  - Webhook 接收
  - API 调用
  - 富文本渲染
  - 卡片消息

---

## Phase 5: 前端开发 (Week 9-10)

### 5.1 Flutter 项目初始化
- 项目配置 (桌面 + Web 平台)
- 依赖注入设置
- GoRouter 路由配置
- Riverpod 状态管理框架

### 5.2 核心界面 (Week 9)
- 桌面分栏布局 (侧边栏 + 主内容区)
- Web 响应式布局适配
- 会话列表页
- 聊天页面 (基础)
- 设置页面框架

### 5.3 连接和消息 (Week 10)
- WebSocket 服务集成
- 消息收发
- Markdown 渲染
- 代码高亮显示
- 会话持久化

### 5.4 桌面端增强 (Week 11)
- 窗口管理 (记忆大小/位置)
- 系统托盘集成
- 快捷键支持
- 菜单条 (文件/编辑/视图/帮助)
- 文件拖拽上传

### 5.5 Web 端和 polish (Week 12)
- PWA 支持 (可安装)
- 主题切换 (亮色/暗色/系统)
- 导出会话功能
- 性能优化
- 测试

---

## Phase 6: 安全和优化 (Week 11-12)

### 6.1 安全模块
- 工作区访问控制
- 网络策略
- 沙箱执行

### 6.2 性能优化
- 上下文压缩
- 缓存策略
- 并发优化

### 6.3 测试和文档
- 单元测试
- 集成测试
- API 文档

---

## 关键数据结构

### 消息结构
```go
type Message struct {
    ID        string
    Role      string // user, assistant, system, tool
    Content   string
    Timestamp time.Time
    Metadata  map[string]any
}
```

### 会话结构
```go
type Session struct {
    ID        string
    CreatedAt time.Time
    UpdatedAt time.Time
    Messages  []Message
    Metadata  map[string]any
}
```

### 工具定义
```go
type Tool struct {
    Name        string
    Description string
    Parameters  JSONSchema
    Handler     func(ctx context.Context, args map[string]any) (any, error)
}
```

---

## API 设计

### WebSocket API
- `/ws` - WebSocket 连接端点

### REST API
- `GET /api/sessions` - 获取会话列表
- `POST /api/sessions` - 创建新会话
- `GET /api/sessions/:id` - 获取会话详情
- `POST /api/sessions/:id/messages` - 发送消息

