#!/bin/bash

echo "===================================="
echo "  NeoRay macOS 构建脚本"
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
echo "[4/5] 构建 macOS 发布版本..."
flutter build macos --release

echo ""
echo "[5/5] 打包 DMG..."
mkdir -p dist
APP_PATH="build/macos/Build/Products/Release/neoray_app.app"
DMG_PATH="dist/neoray-macos.dmg"

if [ -f "$DMG_PATH" ]; then
    rm -f "$DMG_PATH"
fi

if command -v create-dmg &> /dev/null; then
    create-dmg --volname "NeoRay" --volicon "assets/logo.icns" "$APP_PATH" "$DMG_PATH"
else
    hdiutil create -volname "NeoRay" -srcfolder "$APP_PATH" -ov -format UDZO "$DMG_PATH"
fi

echo ""
echo "===================================="
echo "  ✅ 构建成功！"
echo "===================================="
echo "输出位置: $DMG_PATH"
echo "App 路径: $APP_PATH"
