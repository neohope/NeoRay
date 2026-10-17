import { useState } from 'react';

/** 底部输入区 */
export function Composer({
  disabled,
  onSend,
  placeholder,
}: {
  disabled: boolean;
  onSend: (text: string) => void;
  placeholder?: string;
}) {
  const [text, setText] = useState('');

  function submit() {
    const v = text.trim();
    if (!v || disabled) return;
    onSend(v);
    setText('');
  }

  return (
    <div className="composer">
      <textarea
        value={text}
        disabled={disabled}
        placeholder={disabled ? '等待回复中…' : placeholder ?? '输入消息，Enter 发送，Shift+Enter 换行'}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            submit();
          }
        }}
        rows={1}
      />
      <button onClick={submit} disabled={disabled || !text.trim()}>
        {disabled ? '…' : '发送'}
      </button>
    </div>
  );
}
