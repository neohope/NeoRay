import { useEffect, useRef } from 'react';
import type { Message, ToolCall, ToolResult } from '../api/types';

/** 单条消息气泡 */
export function MessageBubble({
  message,
  streaming = false,
}: {
  message: Message;
  streaming?: boolean;
}) {
  const isUser = message.role === 'user';

  return (
    <div className={`bubble-row ${isUser ? 'from-user' : 'from-assistant'}`}>
      <div className={`bubble ${isUser ? 'user' : 'assistant'}`}>
        {!isUser && <div className="bubble-role">NeoRay</div>}
        <div className="bubble-content">
          {message.content || (streaming ? <span className="cursor-blink" /> : '')}
          {streaming && message.content && <span className="cursor-blink" />}
        </div>

        {message.tool_calls?.length ? (
          <ToolCallList calls={message.tool_calls} results={message.tool_results ?? []} />
        ) : null}
      </div>
    </div>
  );
}

/**
 * 流式中的临时气泡：直接使用累积内容与实时工具调用，
 * 不参与 messages 数组，避免每 chunk 触发列表重渲染。
 */
export function StreamingBubble({
  content,
  toolCalls,
  toolResults,
  progressText,
}: {
  content: string;
  toolCalls: ToolCall[];
  toolResults: ToolResult[];
  progressText: string;
}) {
  return (
    <div className="bubble-row from-assistant">
      <div className="bubble assistant">
        <div className="bubble-role">NeoRay</div>

        {progressText && <div className="progress-line">{progressText}</div>}

        {toolCalls.length > 0 && <ToolCallList calls={toolCalls} results={toolResults} />}

        <div className="bubble-content">
          {content ? (
            <>
              {content}
              <span className="cursor-blink" />
            </>
          ) : (
            <span className="thinking">思考中…</span>
          )}
        </div>
      </div>
    </div>
  );
}

/** 工具调用与结果展示 */
function ToolCallList({ calls, results }: { calls: ToolCall[]; results: ToolResult[] }) {
  return (
    <div className="tool-list">
      {calls.map((c) => {
        const result = results.find((r) => r.tool_call_id === c.id);
        const args = prettyArgs(c.arguments);
        return (
          <details key={c.id} className={`tool-item ${result?.is_error ? 'tool-error' : ''}`}>
            <summary>
              <span className="tool-name">🔧 {c.name || 'tool'}</span>
              <span className="tool-state">
                {result ? (result.is_error ? '失败' : '完成') : '执行中…'}
              </span>
            </summary>
            {args && <pre className="tool-args">{args}</pre>}
            {result && <pre className="tool-result">{truncate(result.content, 4000)}</pre>}
          </details>
        );
      })}
    </div>
  );
}

function prettyArgs(raw: string): string {
  if (!raw) return '';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function truncate(s: string, max: number): string {
  if (!s) return '';
  return s.length > max ? `${s.slice(0, max)}\n… (已截断)` : s;
}

/** 自动滚动钩子：内容变化时滚到底部 */
export function useAutoScroll(deps: unknown[]) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    // 仅当用户已在底部附近时自动滚动，避免打断向上翻阅
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 120;
    if (nearBottom) {
      el.scrollTop = el.scrollHeight;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return ref;
}
