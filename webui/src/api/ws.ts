/**
 * WebSocket 客户端封装。
 *
 * 使用 reconnecting-websocket 获得指数退避重连能力，与 Flutter 端语义一致。
 *
 * 鉴权：浏览器 WebSocket API 无法自定义请求头，因此依赖同源 Cookie
 * （登录后由 /api/auth/login 下发，握手时浏览器自动携带）。
 */
import ReconnectingWebSocket, { type Options } from 'reconnecting-websocket';
import type {
  ClientMessageType,
  ServerEnvelope,
  ServerMessageType,
} from './types';

export type EventHandler = (type: ServerMessageType, payload: Record<string, unknown>) => void;

const WS_OPTIONS: Options = {
  maxRetries: 10,
  maxReconnectionDelay: 30000,
  minReconnectionDelay: 1000,
  reconnectionDelayGrowFactor: 1.3,
  connectionTimeout: 10000,
};

export class NeoraySocket {
  private ws: ReconnectingWebSocket | null = null;
  private handlers = new Set<EventHandler>();
  private statusHandlers = new Set<(connected: boolean) => void>();
  private sessionId: string | null = null;

  /** 建立连接（同源 /ws） */
  connect(): void {
    if (this.ws) return;

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${location.host}/ws`;

    this.ws = new ReconnectingWebSocket(url, [], WS_OPTIONS);

    this.ws.addEventListener('open', () => {
      this.statusHandlers.forEach((h) => h(true));
      // 重连后自动重新加入原会话，避免消息丢失订阅
      if (this.sessionId) {
        this.joinSession(this.sessionId);
      }
    });

    this.ws.addEventListener('close', () => {
      this.statusHandlers.forEach((h) => h(false));
    });

    this.ws.addEventListener('message', (ev: MessageEvent) => {
      try {
        const envelope = JSON.parse(ev.data as string) as ServerEnvelope;
        if (!envelope?.type) return;
        const payload = (envelope.payload ?? {}) as Record<string, unknown>;
        this.handlers.forEach((h) => h(envelope.type, payload));
      } catch {
        // 忽略无法解析的消息，避免单条脏数据中断整个连接
      }
    });
  }

  /** 订阅下行事件 */
  onEvent(handler: EventHandler): () => void {
    this.handlers.add(handler);
    return () => this.handlers.delete(handler);
  }

  /** 订阅连接状态变化 */
  onStatus(handler: (connected: boolean) => void): () => void {
    this.statusHandlers.add(handler);
    return () => this.statusHandlers.delete(handler);
  }

  private send(type: ClientMessageType, payload: Record<string, unknown>): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify({ type, payload }));
  }

  /** 发送聊天消息（流式） */
  sendChat(opts: {
    message: string;
    sessionId: string;
    channelId: string;
    userId: string;
  }): void {
    this.send('chat_stream', {
      session_id: opts.sessionId,
      channel_id: opts.channelId,
      user_id: opts.userId,
      message: opts.message,
    });
  }

  /** 加入会话（订阅该会话的推送） */
  joinSession(sessionId: string, channelId?: string, userId?: string): void {
    this.sessionId = sessionId;
    this.send('join_session', {
      session_id: sessionId,
      ...(channelId ? { channel_id: channelId } : {}),
      ...(userId ? { user_id: userId } : {}),
    });
  }

  /** 关闭连接 */
  close(): void {
    this.ws?.close();
    this.ws = null;
  }
}
