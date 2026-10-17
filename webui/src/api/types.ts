/**
 * NeoRay WebSocket / REST 协议类型定义。
 *
 * 本文件是前端与后端协议的唯一事实来源（single source of truth）。
 * 字段名与后端 internal/api 中的结构体 JSON 标签逐一对应，修改时需同步后端。
 */

// ─── REST：会话 ───

/** 会话列表项（GET /api/sessions 返回的 sessions[] 元素） */
export interface SessionSummary {
  id: string;
  channel_id: string;
  user_id: string;
  title: string;
  created_at: string;
  updated_at: string;
  message_count: number;
}

/** 会话详情（GET /api/sessions/:id） */
export interface SessionDetail {
  id: string;
  channel_id: string;
  user_id: string;
  title: string;
  created_at: string;
  updated_at: string;
  messages: Message[];
}

/** 消息（后端 session.Message） */
export interface Message {
  id: string;
  channel_id: string;
  user_id: string;
  session_id: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  timestamp: string;
  tool_calls?: ToolCall[];
  tool_results?: ToolResult[];
}

export interface ToolCall {
  id: string;
  name: string;
  /** JSON 字符串 */
  arguments: string;
}

export interface ToolResult {
  tool_call_id: string;
  content: string;
  is_error?: boolean;
}

// ─── REST：健康检查 ───

export interface HealthResponse {
  status: string;
  time: string;
}

// ─── 认证 ───

export interface LoginResponse {
  ok: boolean;
  expires_in?: number;
  /** 服务端未启用鉴权时为 true */
  auth_disabled?: boolean;
}

export interface AuthMeResponse {
  authenticated: boolean;
  auth_enabled: boolean;
}

// ─── WebSocket：上行（客户端 → 服务端） ───

export type ClientMessageType =
  | 'chat'
  | 'chat_stream'
  | 'create_session'
  | 'join_session'
  | 'list_sessions';

export interface ClientEnvelope {
  type: ClientMessageType;
  payload: Record<string, unknown>;
}

// ─── WebSocket：下行（服务端 → 客户端） ───

/**
 * 下行消息类型。
 *
 * ⚠️ 注意：后端目前不发送 reasoning_start / reasoning_chunk / reasoning_end
 * （Flutter 端枚举里有，但 Go 侧从未发出）。此处不定义这三种类型，
 * 避免前端实现「看起来有、实际从不触发」的功能。
 */
export type ServerMessageType =
  | 'chat_start'
  | 'chat_chunk'
  | 'chat_end'
  | 'tool_call_start'
  | 'tool_call_result'
  | 'session_created'
  | 'session_joined'
  | 'session_list'
  | 'progress'
  | 'error';

export interface ServerEnvelope {
  type: ServerMessageType;
  payload: Record<string, unknown>;
}

/** chat_chunk / chat_end 的 payload */
export interface ChatChunkPayload {
  session_id?: string;
  content?: string;
}

/** error 的 payload */
export interface ErrorPayload {
  code: string;
  message: string;
}

/** progress 的 payload（系统状态提示） */
export interface ProgressPayload {
  session_id?: string;
  content?: string;
}

/** 类型守卫：安全取出字符串字段 */
export function str(v: unknown): string {
  return typeof v === 'string' ? v : '';
}
