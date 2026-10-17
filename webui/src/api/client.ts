/**
 * REST 客户端。
 *
 * 关键点：所有请求带 credentials: 'include'，
 * 使浏览器自动携带 Web UI 会话 Cookie（后端 WS 无法自定义请求头，
 * 因此统一使用同源 Cookie 鉴权）。
 */
import type {
  SessionSummary,
  SessionDetail,
  HealthResponse,
  LoginResponse,
  AuthMeResponse,
} from './types';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = 'ApiError';
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init.headers ?? {}),
    },
  });

  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const data = await res.json();
      msg = data.error ?? data.message ?? msg;
    } catch {
      // 响应非 JSON，保留状态码信息
    }
    throw new ApiError(res.status, msg);
  }

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

// ─── 认证 ───

export function login(key: string): Promise<LoginResponse> {
  return request<LoginResponse>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ key }),
  });
}

export function logout(): Promise<{ ok: boolean }> {
  return request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' });
}

export function authMe(): Promise<AuthMeResponse> {
  return request<AuthMeResponse>('/api/auth/me');
}

// ─── 健康检查 ───

export function health(): Promise<HealthResponse> {
  return request<HealthResponse>('/api/health');
}

// ─── 会话 ───

export async function listSessions(
  channelId: string,
  userId: string,
): Promise<SessionSummary[]> {
  const qs = new URLSearchParams({ channel_id: channelId, user_id: userId });
  const data = await request<{ sessions: SessionSummary[] }>(
    `/api/sessions?${qs.toString()}`,
  );
  return data.sessions ?? [];
}

export function createSession(
  channelId: string,
  userId: string,
  title?: string,
): Promise<SessionDetail> {
  return request<SessionDetail>('/api/sessions', {
    method: 'POST',
    body: JSON.stringify({
      channel_id: channelId,
      user_id: userId,
      name: title ?? '新会话',
    }),
  });
}

export function getSession(
  sessionId: string,
  channelId: string,
  userId: string,
): Promise<SessionDetail> {
  const qs = new URLSearchParams({ channel_id: channelId, user_id: userId });
  return request<SessionDetail>(
    `/api/sessions/${encodeURIComponent(sessionId)}?${qs.toString()}`,
  );
}

export function deleteSession(sessionId: string): Promise<void> {
  return request<void>(`/api/sessions/${encodeURIComponent(sessionId)}`, {
    method: 'DELETE',
  });
}
