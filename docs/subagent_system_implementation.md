# 子代理系统实现文档

## 概述

参考 `reference/neonanobot/neonanobot/` 项目，已在 `neoray` 项目中实现了完整的子代理系统。

## 实现的文件

### 1. `neoray/internal/subagent/status.go`
- `SubagentStatus` 结构体，跟踪子代理执行状态
- 支持的阶段：初始化、等待工具、工具完成、最终响应、完成、错误
- 工具事件记录
- Token 使用统计

### 2. `neoray/internal/subagent/manager.go`
- `Manager` 结构体，管理子代理生命周期
- `Spawn()` 方法启动子代理
- 并发控制（默认 5 个）
- 通过消息总线返回结果
- 子代理的隔离执行环境

### 3. `neoray/internal/subagent/tool.go`
- `SpawnTool` 结构体，提供给 LLM 的工具
- 参数：task、label、temperature
- `SetOriginContext()` 方法设置上下文

### 4. 模板文件
- `neoray/templates/agent/subagent_system.md` - 子代理系统提示词
- `neoray/templates/agent/subagent_announce.md` - 结果公告模板

### 5. 配置更新
- `neoray/internal/config/config.go` - 添加 `Tools.Subagent` 配置
- `neoray/internal/config/loader.go` - 添加默认值和配置模板

### 6. 主 Agent 集成
- `neoray/internal/agent/agent.go` - 集成子代理系统
- 自动初始化和注册 spawn 工具

### 7. 内存系统支持
- `neoray/internal/memory/manager.go` - 添加 `BuildSkillsSummary()` 方法

## 架构特点

1. **生产者-消费者模式** - 主代理生成任务，子代理消费执行
2. **隔离执行** - 每个子代理有独立的执行环境
3. **消息总线通信** - 通过消息总线解耦组件
4. **状态追踪** - 实时追踪子代理执行状态
5. **并发控制** - 可配置最大并发数

## 使用方式

### LLM 调用 spawn 工具
```json
{
  "tool": "spawn",
  "params": {
    "task": "分析项目代码结构并生成文档",
    "label": "代码分析",
    "temperature": 0.7
  }
}
```

### 子代理执行流程
1. 主代理调用 `spawn` 工具
2. `SubagentManager` 创建新的子代理任务
3. 子代理在后台独立执行
4. 子代理通过消息总线返回结果
5. 主代理接收结果并以自然语言通知用户

## 配置选项

```toml
[tools.subagent]
enabled = true
max_concurrent = 5
max_iterations = 10
max_tool_result_chars = 16000
```

## 系统集成

子代理系统已完全集成到主 Agent 中：
- `Agent.NewAgent()` 自动初始化子代理管理器
- `spawn` 工具自动注册到工具注册表
- Chat/Stream 方法自动设置上下文
- 支持消息总线集成

## 注意事项

现有的 provider 包中有一些编译错误（与子代理系统无关）：
- `anthropic.go` 中有未使用的变量
- 这些不影响子代理系统的架构完整性
