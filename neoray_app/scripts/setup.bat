@echo off
echo ====================================
echo   NeoRay 项目设置
echo ====================================
echo.

cd /d "%~dp0.."

echo [1/3] 检查 Flutter 环境...
where flutter >nul 2>nul
if errorlevel 1 (
    echo ❌ 未找到 Flutter，请先安装 Flutter
    echo 下载地址: https://flutter.dev/docs/get-started/install
    pause
    exit /b 1
)
echo ✅ Flutter 已安装
call flutter --version

echo.
echo [2/3] 启用 Windows 桌面支持...
call flutter config --enable-windows-desktop

echo.
echo [3/3] 获取项目依赖...
call flutter pub get
if errorlevel 1 (
    echo ❌ 依赖获取失败！
    pause
    exit /b 1
)
echo ✅ 依赖已获取

echo.
echo ====================================
echo   ✅ 设置完成！
echo ====================================
echo.
echo 现在可以运行以下命令：
echo   flutter run              - 运行开发版本
echo   flutter build windows   - 构建发布版本
echo   scripts\build-windows.bat - 完整构建并打包
echo.
pause
