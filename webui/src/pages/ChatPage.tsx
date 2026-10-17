import { useEffect } from 'react';
import { useSessionStore } from '../stores/sessionStore';
import { useWebSocket } from '../hooks/useWebSocket';
import { Sidebar } from '../components/Sidebar';
import { MessageBubble, StreamingBubble, useAutoScroll } from '../components/MessageBubble';
import { Composer } from '../components/Composer';

/** 聊天主界面 */
export function ChatPage() {
  const {
    sessions,
    currentSessionId,
    messages,
    streaming,
    streamingContent,
    liveToolCalls,
    liveToolResults,
    progressText,
    error,
    loadSessions,
    createSession,
    selectSession,
    deleteSession,
    appendUserMessage,
    setError,
  } = useSessionStore();

  const { connected, sendChat, joinSession } = useWebSocket(true);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  // 切换会话时订阅该会话的 WS 推送
  useEffect(() => {
    if (currentSessionId && connected) {
      joinSession(currentSessionId);
    }
  }, [currentSessionId, connected, joinSession]);

  const scrollRef = useAutoScroll([messages.length, streamingContent, liveToolCalls.length]);

  function handleSend(text: string) {
    if (!currentSessionId) {
      void createSession();
      return;
    }
    setError(null);
    appendUserMessage(text);
    sendChat(text, currentSessionId);
  }

  return (
    <div className="app-layout">
      <Sidebar
        sessions={sessions}
        currentId={currentSessionId}
        onSelect={(id) => void selectSession(id)}
        onCreate={() => void createSession()}
        onDelete={(id) => void deleteSession(id)}
      />

      <main className="main-pane">
        <header className="main-head">
          <span className="conn-dot" data-connected={connected} />
          <span className="head-title">
            {sessions.find((s) => s.id === currentSessionId)?.title ?? '新会话'}
          </span>
          <span className="head-status">{connected ? '已连接' : '重连中…'}</span>
        </header>

        {error && (
          <div className="banner-error" onClick={() => setError(null)}>
            {error} <span className="banner-close">×</span>
          </div>
        )}

        <div className="messages" ref={scrollRef}>
          {!currentSessionId && messages.length === 0 && (
            <div className="welcome">
              <div className="welcome-icon">🐰</div>
              <p>开始一个新会话吧</p>
              <button onClick={() => void createSession()}>新建会话</button>
            </div>
          )}

          {messages.map((m) => (
            <MessageBubble key={m.id} message={m} />
          ))}

          {streaming && (
            <StreamingBubble
              content={streamingContent}
              toolCalls={liveToolCalls}
              toolResults={liveToolResults}
              progressText={progressText}
            />
          )}
        </div>

        <Composer
          disabled={streaming || !currentSessionId}
          onSend={handleSend}
          placeholder={currentSessionId ? undefined : '请先新建会话'}
        />
      </main>
    </div>
  );
}
