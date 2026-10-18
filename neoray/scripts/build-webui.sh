#!/bin/bash
# 构建 Web UI 前端资源，产物由 go:embed 打包进 neoray 二进制。
#
# 设计原则：
#   - 前端资源是「可选增强」：构建失败不阻断后端发布，
#     此时服务端会回退到内置提示页。
#   - 未安装 Node 时给出明确提示并跳过，而不是报错退出。
#
# 用法：./scripts/build-webui.sh (从 neoray/ 目录调用)

set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
WEBUI_DIR="$ROOT_DIR/webui"

echo "=== 构建 Web UI ==="

if [ ! -d "$WEBUI_DIR" ]; then
    echo "⚠️  未找到 $WEBUI_DIR，跳过 Web UI 构建"
    echo "    （服务端将使用内置提示页，不影响其他功能）"
    exit 0
fi

if ! command -v npm > /dev/null 2>&1; then
    echo "⚠️  未找到 npm，跳过 Web UI 构建"
    echo "    如需 Web UI，请安装 Node.js 后重新构建。"
    exit 0
fi

cd "$WEBUI_DIR" || exit 0

if [ ! -d node_modules ]; then
    echo "[1/2] 安装前端依赖..."
    if ! npm install; then
        echo "⚠️  依赖安装失败，跳过 Web UI 构建"
        exit 0
    fi
else
    echo "[1/2] 前端依赖已存在，跳过安装"
fi

echo "[2/2] 构建前端资源..."
if npm run build; then
    echo "✅ Web UI 构建完成，产物将嵌入 Go 二进制"
else
    echo "⚠️  Web UI 构建失败，将使用内置提示页（不影响后端功能）"
fi

exit 0
