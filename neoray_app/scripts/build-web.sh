#!/bin/bash

echo "===================================="
echo "  NeoRay 前端 Web 构建脚本"
echo "===================================="
echo ""

cd "$(dirname "$0")/.."

echo "[1/5] 清理构建缓存..."
flutter clean

echo ""
echo "[2/5] 检查 Flutter 环境..."
if ! command -v flutter &> /dev/null; then
    echo "❌ 未找到 Flutter，请先安装 Flutter"
    echo "下载地址: https://flutter.dev/docs/get-started/install"
    exit 1
fi
echo "✅ Flutter 已安装"
flutter --version

echo ""
echo "[3/5] 获取依赖..."
flutter pub get
if [ $? -ne 0 ]; then
    echo "❌ 依赖获取失败！"
    exit 1
fi
echo "✅ 依赖已获取"

echo ""
echo "[4/5] 生成代码..."
flutter pub run build_runner build --delete-conflicting-outputs

echo ""
echo "[5/5] 构建 Web 版本..."
flutter build web --release --web-renderer canvaskit
if [ $? -ne 0 ]; then
    echo "❌ Web 构建失败！"
    exit 1
fi
echo "✅ Web 构建成功"

echo ""
echo "===================================="
echo "  ✅ 构建成功！"
echo "===================================="
echo ""
echo "输出位置: build/web/"
echo ""
echo "本地测试方式:"
echo "  1. cd build/web"
echo "  2. python3 -m http.server 8080"
echo "  3. 浏览器访问 http://localhost:8080"
echo ""
echo "或使用 Flutter 内置服务器:"
echo "  flutter run -d chrome"
echo ""
