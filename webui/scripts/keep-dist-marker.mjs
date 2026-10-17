/**
 * 构建后恢复 dist/.gitkeep。
 *
 * 背景：vite 的 build.emptyOutDir 会清空输出目录，而该目录需要保留一个
 * 被版本库跟踪的文件，否则全新克隆后 `go build` 会因 go:embed 找不到
 * 目标目录而失败。此处用 Node 实现（跨平台，不依赖 shell 的 touch）。
 */
import { writeFileSync, existsSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const distDir = join(here, '..', '..', 'neoray', 'internal', 'webui', 'dist');

if (!existsSync(distDir)) {
  mkdirSync(distDir, { recursive: true });
}

writeFileSync(join(distDir, '.gitkeep'), '');
console.log('✓ 已恢复 neoray/internal/webui/dist/.gitkeep');
