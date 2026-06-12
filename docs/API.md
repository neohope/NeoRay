# NeoRay API 文档

## 概述

NeoRay 提供 REST API 和 WebSocket 接口用于与 AI 助手交互。

### 新功能 (v2)

- **流式工具调用**: WebSocket 流式响应支持实时工具调用反馈
- **并行工具执行**: 多个工具调用会并行执行以提高响应速度
- **Token 管理**: 内置 Token 使用统计和预算控制
- **执行追踪**: 详细的执行过程追踪用于调试
- **智能上下文管理**: 多种上下文截断策略（最近消息、摘要、重要性）

## 基础配置

默认服务地址: `http://localhost:8080`

## REST API

### 健康检查
```
GET /api/health
```

响应示例:
```json
{
  "status": "ok",
  "time": "2024-01-01T00:00:00Z"
}
```

### 会话列表
```
GET /api/sessions
```

响应示例:
```json
{
  "sessions": [
    {
      "id": "abc123",
      "name": "My Chat",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "message_count": 5
    }
  ]
}
```

### 创建会话
```
POST /api/sessions
Content-Type: application/json

{
  "name": "My New Chat"
}
```

响应示例:
```json
{
  "id": "abc123",
  "name": "My New Chat",
  "created_at": "2024-01-01T00:00:00Z"
}
```

### 获取会话详情
```
GET /api/sessions/{session_id}
```

### 发送聊天消息
```
POST /api/sessions/{session_id}
Content-Type: application/json

{
  "message": "Hello!",
  "stream": false
}
```

- `stream`: 是否使用流式响应（默认 false）

非流式响应示例:
```json
{
  "content": "Hello! How can I help you?",
  "token_usage": {
    "input_tokens": 100,
    "output_tokens": 50,
    "total_tokens": 150
  },
  "tool_calls": 2
}
```

流式响应 (SSE 格式):
```
data: {"type":"text","content":"Hello"}

data: {"type":"tool_start","tool_calls":[...]}

data: {"type":"tool_result","tool_result":[...]}

data: {"type":"end","content":"Hello! How can I help you?"}
```

### 删除会话
```
DELETE /api/sessions/{session_id}
```

## WebSocket API

连接地址: `ws://localhost:8080/ws`

### 消息格式

所有消息遵循以下格式:
```json
{
  "type": "message_type",
  "payload": {}
}
```

### 客户端 -> 服务器 消息

#### chat - 发送聊天消息
```json
{
  "type": "chat",
  "payload": {
    "session_id": "abc123",
    "message": "Hello!"
  }
}
```

#### chat_stream - 发送流式聊天消息
```json
{
  "type": "chat_stream",
  "payload": {
    "session_id": "abc123",
    "message": "Hello!"
  }
}
```

#### create_session - 创建会话
```json
{
  "type": "create_session",
  "payload": {
    "name": "My Chat"
  }
}
```

#### join_session - 加入会话
```json
{
  "type": "join_session",
  "payload": {
    "session_id": "abc123"
  }
}
```

#### list_sessions - 列出会话
```json
{
  "type": "list_sessions"
}
```

### 服务器 -> 客户端 消息

#### chat_start - 聊天开始
```json
{
  "type": "chat_start",
  "payload": {
    "session_id": "abc123"
  }
}
```

#### chat_chunk - 流式内容块
```json
{
  "type": "chat_chunk",
  "payload": {
    "session_id": "abc123",
    "content": "Hello"
  }
}
```

#### chat_end - 聊天结束
```json
{
  "type": "chat_end",
  "payload": {
    "session_id": "abc123",
    "content": "Hello! How can I help you?",
    "token_usage": {
      "input_tokens": 100,
      "output_tokens": 50,
      "total_tokens": 150
    },
    "tool_calls": 2,
    "iterations": 3
  }
}
```

#### tool_call_start - 工具调用开始 (流式)
当 AI 决定使用工具时会发送此消息。
```json
{
  "type": "tool_call_start",
  "payload": {
    "session_id": "abc123",
    "tool_calls": [
      {
        "id": "call_123",
        "name": "search_files",
        "arguments": "{\"pattern\":\"*.go\"}"
      }
    ]
  }
}
```

#### tool_call_result - 工具调用结果 (流式)
工具执行完成后会发送此消息。
```json
{
  "type": "tool_call_result",
  "payload": {
    "session_id": "abc123",
    "tool_result": [
      {
        "tool_use_id": "call_123",
        "content": "Found 5 files..."
      }
    ]
  }
}
```

#### session_created - 会话已创建
```json
{
  "type": "session_created",
  "payload": {
    "session_id": "abc123",
    "name": "My Chat",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### session_joined - 已加入会话
```json
{
  "type": "session_joined",
  "payload": {
    "session_id": "abc123",
    "name": "My Chat",
    "messages": [...]
  }
}
```

#### session_list - 会话列表
```json
{
  "type": "session_list",
  "payload": {
    "sessions": [...]
  }
}
```

#### error - 错误
```json
{
  "type": "error",
  "payload": {
    "code": "error_code",
    "message": "Error message"
  }
}
```

## Feishu (飞书) 集成

### 配置

在配置文件中设置:
```toml
[channels.feishu]
enabled = true
app_id = "cli_..."
app_secret = "..."
verification_token = "..."
webhook_path = "/webhook/feishu"
```

### Webhook

飞书事件会发送到配置的 webhook 路径。

支持的事件:
- `im.message.receive_v1` - 接收消息

### 消息处理

- 用户在飞书中发送消息
- NeoRay 自动创建/关联会话
- AI 回复自动发送回飞书

## CORS

API 已启用 CORS，允许跨域请求。

## WebSocket 心跳

- 服务器每 30 秒发送 ping
- 客户端需在 60 秒内响应 pong
- 超时连接会被关闭
