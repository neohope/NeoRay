/**
 * 会话与消息状态管理（Zustand）。
 *
 * 设计要点：
 * - 流式内容不写入 messages 数组，而是单独放在 streamingContent，
 *   避免每个 chunk 都复制整个消息列表（与 Flutter 端同样的性能考量）。
 * - 流式结束（chat_end）时才把最终内容合并为一条 assistant 消息追加到列表。
 */
import { create } from 'zustand';
import type { Message, SessionSummary, ToolCall, ToolResult } from '../api/types';
import * as api from '../api/client';

/** Web UI 使用的固定频道标识 */
export const WEB_CHANNEL_ID = 'webui';

const USER_ID_STORAGE_KEY = 'neoray_user_id';

/** 生成/读取持久化的 userId（浏览器本地，非敏感） */
export function getUserId(): string {
  let id = localStorage.getItem(USER_ID_STORAGE_KEY);
  if (!id) {
    id = `web-${Math.random().toString(36).slice(2, 10)}`;
    localStorage.setItem(USER_ID_STORAGE_KEY, id);
  }
  return id;
}

interface SessionState {
  sessions: SessionSummary[];
  currentSessionId: string | null;
  messages: Message[];

  /** 是否为流式输出中 */
  streaming: boolean;
  /** 流式累积内容（chat_chunk） */
  streamingContent: string;
  /** 当前轮的工具调用（进行中/已完成） */
  liveToolCalls: ToolCall[];
  liveToolResults: ToolResult[];
  /** 系统状态提示（progress 事件） */
  progressText: string;

  loading: boolean;
  error: string | null;

  // ─── actions ───
  loadSessions: () => Promise<void>;
  createSession: () => Promise<void>;
  selectSession: (id: string) => Promise<void>;
  deleteSession: (id: string) => Promise<void>;

  appendUserMessage: (content: string) => void;
  startStreaming: () => void;
  appendChunk: (chunk: string) => void;
  finishStreaming: () => void;
  addToolCall: (tc: ToolCall) => void;
  addToolResult: (tr: ToolResult) => void;
  setProgress: (text: string) => void;
  failStreaming: (message: string) => void;
  setError: (message: string | null) => void;
  renameCurrent: (title: string) => void;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  sessions: [],
  currentSessionId: null,
  messages: [],
  streaming: false,
  streamingContent: '',
  liveToolCalls: [],
  liveToolResults: [],
  progressText: '',
  loading: false,
  error: null,

  async loadSessions() {
    set({ loading: true });
    try {
      const sessions = await api.listSessions(WEB_CHANNEL_ID, getUserId());
      set({ sessions, loading: false });
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },

  async createSession() {
    try {
      const created = await api.createSession(WEB_CHANNEL_ID, getUserId());
      await get().loadSessions();
      set({
        currentSessionId: created.id,
        messages: [],
        streamingContent: '',
        liveToolCalls: [],
        liveToolResults: [],
        progressText: '',
      });
    } catch (e) {
      set({ error: (e as Error).message });
    }
  },

  async selectSession(id) {
    set({ loading: true, error: null });
    try {
      const detail = await api.getSession(id, WEB_CHANNEL_ID, getUserId());
      set({
        currentSessionId: id,
        messages: detail.messages ?? [],
        streamingContent: '',
        liveToolCalls: [],
        liveToolResults: [],
        progressText: '',
        loading: false,
      });
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },

  async deleteSession(id) {
    try {
      await api.deleteSession(id);
      const wasCurrent = get().currentSessionId === id;
      await get().loadSessions();
      if (wasCurrent) {
        set({ currentSessionId: null, messages: [] });
      }
    } catch (e) {
      set({ error: (e as Error).message });
    }
  },

  appendUserMessage(content) {
    const { currentSessionId, messages } = get();
    if (!currentSessionId) return;
    const msg: Message = {
      id: `local-${Date.now()}`,
      channel_id: WEB_CHANNEL_ID,
      user_id: getUserId(),
      session_id: currentSessionId,
      role: 'user',
      content,
      timestamp: new Date().toISOString(),
    };
    set({ messages: [...messages, msg] });
  },

  startStreaming() {
    set({ streaming: true, streamingContent: '', liveToolCalls: [], liveToolResults: [], progressText: '' });
  },

  appendChunk(chunk) {
    // 累积到独立字段，不触碰 messages，避免 O(n) 列表拷贝
    set({ streamingContent: get().streamingContent + chunk });
  },

  finishStreaming() {
    const { streamingContent, messages, currentSessionId, liveToolCalls, liveToolResults } = get();
    if (!currentSessionId) return;

    const next = [...messages];
    if (streamingContent) {
      next.push({
        id: `local-a-${Date.now()}`,
        channel_id: WEB_CHANNEL_ID,
        user_id: getUserId(),
        session_id: currentSessionId,
        role: 'assistant',
        content: streamingContent,
        timestamp: new Date().toISOString(),
        tool_calls: liveToolCalls.length ? liveToolCalls : undefined,
        tool_results: liveToolResults.length ? liveToolResults : undefined,
      });
    }

    set({
      messages: next,
      streaming: false,
      streamingContent: '',
      liveToolCalls: [],
      liveToolResults: [],
      progressText: '',
    });

    // 刷新会话列表：否则侧边栏的 message_count 会停留在旧值，
    // 与聊天区实际消息数不一致。
    void get().loadSessions();
  },

  addToolCall(tc) {
    set({ liveToolCalls: [...get().liveToolCalls, tc] });
  },

  addToolResult(tr) {
    set({ liveToolResults: [...get().liveToolResults, tr] });
  },

  setProgress(text) {
    set({ progressText: text });
  },

  failStreaming(message) {
    set({
      streaming: false,
      streamingContent: '',
      progressText: '',
      error: message,
    });
  },

  setError(message) {
    set({ error: message });
  },

  renameCurrent(title) {
    const { currentSessionId, sessions } = get();
    if (!currentSessionId) return;
    set({
      sessions: sessions.map((s) => (s.id === currentSessionId ? { ...s, title } : s)),
    });
  },
}));
