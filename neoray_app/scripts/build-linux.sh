#!/bin/bash

echo "===================================="
echo "  NeoRay Linux 构建脚本"
echo "===================================="
echo ""

cd "$(dirname "$0")/.."

echo "[1/5] 清理构建缓存..."
flutter clean

echo ""
echo "[2/5] 获取依赖..."
flutter pub get

echo ""
echo "[3/5] 生成代码..."
flutter pub run build_runner build --delete-conflicting-outputs

echo ""
echo "[4/5] 构建 Linux 发布版本..."
flutter build linux --release

echo ""
echo "[5/5] 打包..."
mkdir -p dist

# 打包 tar.gz
APP_PATH="build/linux/x64/release/bundle"
TAR_PATH="dist/neoray-linux.tar.gz"

if [ -f "$TAR_PATH" ]; then
    rm -f "$TAR_PATH"
fi

cd "$APP_PATH"
tar -czf "../../../../../"$TAR_PATH .""
cd "../../../../../.."

echo ""
echo "===================================="
echo "  ✅ 构建成功！"
echo "===================================="
echo "输出位置: $TAR_PATH"
echo "可执行文件: $APP_PATH/neoray_app"
