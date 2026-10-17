/**
 * WebSocket 连接管理与事件分发。
 *
 * 从 UI 组件中抽离，使其可独立测试，也避免在页面组件里堆积业务逻辑。
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import { NeoraySocket } from '../api/ws';
import { useSessionStore, getUserId, WEB_CHANNEL_ID } from '../stores/sessionStore';
import { str } from '../api/types';

export function useWebSocket(enabled: boolean) {
  const socketRef = useRef<NeoraySocket | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!enabled) return;

    const socket = new NeoraySocket();
    socketRef.current = socket;

    const offStatus = socket.onStatus(setConnected);

    const offEvent = socket.onEvent((type, payload) => {
      const store = useSessionStore.getState();
      // 只处理当前会话的事件，避免串台
      const sid = str(payload.session_id);
      const current = store.currentSessionId;
      if (sid && current && sid !== current) return;

      switch (type) {
        case 'chat_start':
          store.startStreaming();
          break;

        case 'chat_chunk':
          store.appendChunk(str(payload.content));
          break;

        case 'chat_end': {
          const finalContent = str(payload.content);
          // 若服务端在 chat_end 携带完整内容，用它覆盖累积值
          if (finalContent && finalContent.length > store.streamingContent.length) {
            useSessionStore.setState({ streamingContent: finalContent });
          }
          store.finishStreaming();
          break;
        }

        case 'tool_call_start': {
          const calls = payload.tool_calls;
          if (Array.isArray(calls)) {
            calls.forEach((c) => {
              const item = c as Record<string, unknown>;
              store.addToolCall({
                id: str(item.id),
                name: str(item.name),
                arguments: str(item.arguments),
              });
            });
          }
          break;
        }

        case 'tool_call_result': {
          const results = payload.tool_result;
          if (Array.isArray(results)) {
            results.forEach((r) => {
              const item = r as Record<string, unknown>;
              store.addToolResult({
                tool_call_id: str(item.tool_call_id),
                content: str(item.content),
                is_error: Boolean(item.is_error),
              });
            });
          } else if (results) {
            const item = results as Record<string, unknown>;
            store.addToolResult({
              tool_call_id: str(item.tool_call_id),
              content: str(item.content),
              is_error: Boolean(item.is_error),
            });
          }
          break;
        }

        case 'progress':
          store.setProgress(str(payload.content));
          break;

        case 'error': {
          const msg = str(payload.message) || str(payload.error) || '服务端返回错误';
          store.failStreaming(msg);
          break;
        }

        case 'session_created':
        case 'session_joined':
        case 'session_list':
          // 会话元数据以 REST 为准，WS 事件仅作触发刷新
          void store.loadSessions();
          break;

        default:
          break;
      }
    });

    socket.connect();

    return () => {
      offEvent();
      offStatus();
      socket.close();
      socketRef.current = null;
    };
  }, [enabled, setConnected]);

  // 用 useCallback 稳定引用：这些函数会进入调用方的 useEffect 依赖数组，
  // 若每次渲染重建会导致依赖变化 → 无限重复调用。
  const sendChat = useCallback((message: string, sessionId: string) => {
    socketRef.current?.sendChat({
      message,
      sessionId,
      channelId: WEB_CHANNEL_ID,
      userId: getUserId(),
    });
  }, []);

  const joinSession = useCallback((sessionId: string) => {
    socketRef.current?.joinSession(sessionId, WEB_CHANNEL_ID, getUserId());
  }, []);

  return { connected, sendChat, joinSession };
}
