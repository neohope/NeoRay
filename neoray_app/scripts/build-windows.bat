@echo off
echo ====================================
echo   NeoRay Windows 构建脚本
echo ====================================
echo.

cd /d "%~dp0.."

echo [1/5] 清理构建缓存...
call flutter clean
if errorlevel 1 (
    echo 清理失败！
    exit /b 1
)

echo.
echo [2/5] 获取依赖...
call flutter pub get
if errorlevel 1 (
    echo 依赖获取失败！
    exit /b 1
)

echo.
echo [3/5] 生成代码...
call flutter pub run build_runner build --delete-conflicting-outputs
if errorlevel 1 (
    echo 代码生成失败！
    exit /b 1
)

echo.
echo [4/5] 构建 Windows 发布版本...
call flutter build windows --release
if errorlevel 1 (
    echo 构建失败！
    exit /b 1
)

echo.
echo [5/5] 打包 zip...
if not exist "dist" mkdir dist
powershell -Command "if (Test-Path 'dist\neoray-windows.zip') { Remove-Item 'dist\neoray-windows.zip' -Force }"
powershell -Command "Compress-Archive -Path 'build\windows\x64\runner\Release\*' -DestinationPath 'dist\neoray-windows.zip' -Force"

if errorlevel 1 (
    echo 打包失败！
    exit /b 1
)

echo.
echo ====================================
echo   ✅ 构建成功！
echo ====================================
echo 输出位置: dist\neoray-windows.zip
echo 可执行文件: build\windows\x64\runner\Release\neoray_app.exe
echo.
