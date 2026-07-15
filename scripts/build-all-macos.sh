#!/bin/bash

echo "===================================="
echo "  NeoRay 完整构建脚本 (macOS)"
echo "===================================="
echo ""

cd "$(dirname "$0")/.."

echo "[1/2] 构建后端..."
cd neoray
chmod +x scripts/build-macos.sh
./scripts/build-macos.sh
if [ $? -ne 0 ]; then
    echo "❌ 后端构建失败！"
    exit 1
fi
cd ..

echo ""
echo "[2/2] 构建前端..."
cd neoray_app
chmod +x scripts/build-macos.sh
./scripts/build-macos.sh
if [ $? -ne 0 ]; then
    echo "❌ 前端构建失败！"
    exit 1
fi
cd ..

echo ""
echo "===================================="
echo "  ✅ 全部构建成功！"
echo "===================================="
echo ""
echo "后端输出: neoray/dist/"
echo "前端输出: neoray_app/dist/"
echo ""
echo "运行方式:"
echo "  1. 后端: cd neoray/build && ./neoray --no-tui"
echo "  2. 前端: cd neoray_app && flutter run"
echo ""
