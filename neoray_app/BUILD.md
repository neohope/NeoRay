# NeoRay 打包指南

## 前置准备

### 1. 启用平台支持

```bash
# 桌面平台
flutter config --enable-windows-desktop
flutter config --enable-macos-desktop
flutter config --enable-linux-desktop

# Web 平台（默认已启用）
flutter devices  # 确认 Chrome 可用
```

### 2. 验证环境

```bash
flutter doctor
```

## 代码生成

运行代码生成器（Freezed/JSON/Hive）：

```bash
cd neoray_app
flutter pub run build_runner build --delete-conflicting-outputs
```

## Windows 打包

### 1. 构建发布版本

```bash
flutter build windows --release
```

### 2. 生成 MSIX 安装包（推荐）

先激活 msix 包：

```bash
dart pub global activate msix
```

编辑 `pubspec.yaml`，添加 msix 配置：

```yaml
msix_config:
  display_name: NeoRay
  publisher_display_name: NeoRay Team
  identity_name: com.neoray.app
  msix_version: 1.0.0.0
  logo_path: assets/logo.png
  capabilities: internetClient
```

构建 MSIX：

```bash
dart pub global run msix:create
```

### 3. 或者创建可分发的 zip 包

```bash
# 输出在 build/windows/x64/runner/Release/
powershell Compress-Archive -Path build/windows/x64/runner/Release/* -DestinationPath neoray-windows.zip
```

## macOS 打包

### 1. 构建发布版本

```bash
flutter build macos --release
```

### 2. 打包 .app

```bash
# 输出在 build/macos/Build/Products/Release/NeoRay.app
hdiutil create -volname NeoRay -srcfolder build/macos/Build/Products/Release/NeoRay.app -ov -format UDZO neoray-macos.dmg
```

## Linux 打包

### 1. 构建发布版本

```bash
flutter build linux --release
```

### 2. 打包 AppImage

```bash
# 使用 flutter_appimage
cd build/linux/x64/release/bundle/
wget https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
chmod +x appimagetool-x86_64.AppImage
./appimagetool-x86_64.AppImage .
```

### 3. 或者打包 deb/rpm

参考：https://docs.flutter.dev/deployment/linux

## Web 打包

### 1. 构建 Web 版本

```bash
flutter build web --release
```

可选参数：
- `--web-renderer canvaskit`：使用 CanvasKit 渲染器（推荐，性能更好）
- `--web-renderer html`：使用 HTML 渲染器（兼容性更好）

### 2. 本地测试

```bash
# 方式 1：使用 Python
cd build/web
python3 -m http.server 8080
# 浏览器访问 http://localhost:8080

# 方式 2：使用 Flutter 直接运行
flutter run -d chrome
```

### 3. 部署到 Web 服务器

将 `build/web` 目录的内容上传到任何静态文件服务器（Nginx、Apache、Netlify、Vercel、GitHub Pages 等）。

示例 Nginx 配置：
```nginx
server {
    listen 80;
    server_name your-domain.com;
    root /path/to/build/web;
    index index.html;
    
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

## 完整构建脚本

我已为你创建了构建脚本，运行：

```bash
# Windows
./scripts/build-windows.bat
./scripts/build-web.bat          # Web 版本

# macOS
./scripts/build-macos.sh
./scripts/build-web.sh           # Web 版本

# Linux
./scripts/build-linux.sh
./scripts/build-web.sh           # Web 版本
```

或使用 Makefile：
```bash
make windows      # Windows 桌面
make macos        # macOS 桌面
make linux        # Linux 桌面
make web          # Web 版本
make run-web      # 运行 Web 开发版本
```

## 快速开发测试

```bash
# 直接运行（调试模式）
flutter run

# 运行发布模式
flutter run --release
```

## 常见问题

### 1. 依赖问题

```bash
# 清理并重新获取依赖
flutter clean
flutter pub get
```

### 2. 代码生成问题

```bash
# 删除 .dart_tool 并重新生成
rm -rf .dart_tool
flutter pub run build_runner build --delete-conflicting-outputs
```

### 3. 权限问题（Linux）

```bash
chmod +x build/linux/x64/release/bundle/neoray
```
