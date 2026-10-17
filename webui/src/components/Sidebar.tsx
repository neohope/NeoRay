import type { SessionSummary } from '../api/types';

/** 左侧会话列表 */
export function Sidebar({
  sessions,
  currentId,
  onSelect,
  onCreate,
  onDelete,
}: {
  sessions: SessionSummary[];
  currentId: string | null;
  onSelect: (id: string) => void;
  onCreate: () => void;
  onDelete: (id: string) => void;
}) {
  return (
    <aside className="sidebar">
      <div className="sidebar-head">
        <span className="brand">🐰 NeoRay</span>
        <button className="btn-new" onClick={onCreate} title="新建会话">
          ＋
        </button>
      </div>

      <div className="session-list">
        {sessions.length === 0 && <div className="session-empty">暂无会话</div>}

        {sessions.map((s) => (
          <div
            key={s.id}
            className={`session-item ${s.id === currentId ? 'active' : ''}`}
            onClick={() => onSelect(s.id)}
          >
            <div className="session-title">{s.title || '未命名会话'}</div>
            <div className="session-meta">{s.message_count} 条消息</div>
            <button
              className="session-del"
              title="删除会话"
              onClick={(e) => {
                e.stopPropagation();
                if (confirm(`确定删除会话「${s.title || '未命名会话'}」？`)) {
                  onDelete(s.id);
                }
              }}
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </aside>
  );
}
