@echo off
echo ====================================
echo   NeoRay 完整构建脚本 (Windows)
echo ====================================
echo.

cd /d "%~dp0.."

echo [1/2] 构建后端...
cd neoray
call scripts\build-windows.bat
if errorlevel 1 (
    echo ❌ 后端构建失败！
    pause
    exit /b 1
)
cd ..

echo.
echo [2/2] 构建前端...
cd neoray_app
call scripts\build-windows.bat
if errorlevel 1 (
    echo ❌ 前端构建失败！
    pause
    exit /b 1
)
cd ..

echo.
echo ====================================
echo   ✅ 全部构建成功！
echo ====================================
echo.
echo 后端输出: neoray\dist\
echo 前端输出: neoray_app\dist\
echo.
echo 运行方式:
echo   1. 后端: cd neoray\build ^&^& neoray-server.exe --no-tui
echo   2. 前端: cd neoray_app ^&^& flutter run
echo.
pause
