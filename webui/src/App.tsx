import { useEffect } from 'react';
import { useAuthStore } from './stores/authStore';
import { LoginPage } from './pages/LoginPage';
import { ChatPage } from './pages/ChatPage';

export default function App() {
  const { authenticated, checking, check } = useAuthStore();

  // 启动时检查登录态（依据服务端 Cookie）
  useEffect(() => {
    void check();
  }, [check]);

  if (checking) {
    return (
      <div className="boot">
        <div className="boot-spinner" />
      </div>
    );
  }

  return authenticated ? <ChatPage /> : <LoginPage />;
}
