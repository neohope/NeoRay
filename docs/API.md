# NeoRay API 文档

## 概述

NeoRay 提供 REST API 和 WebSocket 接口用于与 AI 助手交互。

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
  "content": "Hello! How can I help you?"
}
```

流式响应: 直接返回文本流

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
    "content": "Hello! How can I help you?"
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
