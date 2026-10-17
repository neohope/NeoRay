import { useEffect } from 'react';
import { useAuthStore } from '../stores/authStore';
import { useState } from 'react';

/**
 * 登录页：输入服务端密钥换取会话 Cookie。
 *
 * 密钥来源：neoray 首次以 web.enabled=true 启动时会自动生成并打印，
 * 同时写入 neoray.toml 的 [security.auth] secret_key。
 */
export function LoginPage() {
  const { login, error, authEnabled } = useAuthStore();
  const [key, setKey] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // 服务端未启用鉴权时直接放行进主界面
  useEffect(() => {
    if (!authEnabled) {
      void login('');
    }
  }, [authEnabled, login]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!key.trim() || submitting) return;
    setSubmitting(true);
    const ok = await login(key.trim());
    if (!ok) setKey('');
    setSubmitting(false);
  }

  return (
    <div className="login-wrap">
      <form className="login-card" onSubmit={handleSubmit}>
        <h1>🐰 NeoRay</h1>
        <p className="login-sub">请输入访问密钥以继续</p>

        <input
          type="password"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          placeholder="secret key"
          autoFocus
          autoComplete="current-password"
        />

        {error && <div className="login-error">{error}</div>}

        <button type="submit" disabled={submitting || !key.trim()}>
          {submitting ? '验证中…' : '登录'}
        </button>

        <p className="login-hint">
          密钥在服务端首次启动时生成并打印在控制台，
          同时保存在 <code>neoray.toml</code> 的
          <code>[security.auth] secret_key</code>。
        </p>
      </form>
    </div>
  );
}
