/**
 * 认证状态管理。
 *
 * 登录态以服务端 Cookie 为准，前端只缓存一个「是否已登录」的镜像。
 */
import { create } from 'zustand';
import * as api from '../api/client';

interface AuthState {
  /** 是否已通过认证 */
  authenticated: boolean;
  /** 服务端是否启用了鉴权 */
  authEnabled: boolean;
  /** 是否正在检查 */
  checking: boolean;
  error: string | null;

  check: () => Promise<void>;
  login: (key: string) => Promise<boolean>;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  authenticated: false,
  authEnabled: true,
  checking: true,
  error: null,

  async check() {
    set({ checking: true });
    try {
      const res = await api.authMe();
      set({
        authenticated: res.authenticated,
        authEnabled: res.auth_enabled,
        checking: false,
        error: null,
      });
    } catch {
      // /api/auth/me 需鉴权，未登录时返回 401 属正常情况
      set({ authenticated: false, authEnabled: true, checking: false });
    }
  },

  async login(key) {
    set({ error: null });
    try {
      await api.login(key);
      set({ authenticated: true });
      return true;
    } catch (e) {
      const err = e as api.ApiError;
      set({
        error: err.status === 401 ? '密钥不正确' : err.message,
        authenticated: false,
      });
      return false;
    }
  },

  async logout() {
    try {
      await api.logout();
    } finally {
      set({ authenticated: false });
    }
  },
}));
