import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

// 构建产物直接输出到 Go 模块内部，供 go:embed 打包进二进制。
// 注意：go:embed 不允许跨出模块边界（不能使用 ../），因此产物必须落在
// neoray/internal/webui/dist/ 之下。
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: fileURLToPath(new URL('../neoray/internal/webui/dist', import.meta.url)),
    emptyOutDir: true,
    // 产物文件名带内容哈希，便于 Go 侧设置长缓存
    assetsDir: 'assets',
  },
  server: {
    // 开发模式：代理到本地后端，避免 CORS 与 Cookie 跨域问题
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: false,
      },
      '/ws': {
        target: 'ws://127.0.0.1:8080',
        ws: true,
      },
    },
  },
});
