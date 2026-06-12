@echo off
echo ====================================
echo   NeoRay 后端 Windows 构建脚本
echo ====================================
echo.

cd /d "%~dp0.."

echo [1/6] 清理构建缓存...
if exist build rmdir /s /q build
if exist dist rmdir /s /q dist

echo [2/6] 检查 Go 环境...
where go >nul 2>nul
if errorlevel 1 (
    echo ❌ 未找到 Go，请先安装 Go
    echo 下载地址: https://golang.org/dl/
    pause
    exit /b 1
)
echo ✅ Go 已安装
call go version

echo.
echo [3/6] 获取依赖...
call go mod tidy
if errorlevel 1 (
    echo ❌ 依赖获取失败！
    pause
    exit /b 1
)
echo ✅ 依赖已获取

echo.
echo [4/6] 运行测试...
call go test ./... -v
if errorlevel 1 (
    echo ⚠️  测试失败，但继续构建...
) else (
    echo ✅ 测试通过
)

echo.
echo [5/6] 构建 Windows 可执行文件...
mkdir build 2>nul
set GOOS=windows
set GOARCH=amd64
call go build -ldflags "-s -w" -o build/neoray-server.exe ./cmd/server
if errorlevel 1 (
    echo ❌ 构建失败！
    pause
    exit /b 1
)
echo ✅ 构建成功

echo.
echo [6/6] 打包...
mkdir dist 2>nul

REM 复制配置文件
copy config.yaml.example build\config.yaml.example 2>nul
copy config.yaml build\config.yaml 2>nul || copy config.yaml.example build\config.yaml

REM 创建发布包
powershell -Command "Compress-Archive -Path build\* -DestinationPath dist\neoray-server-windows.zip -Force"

echo.
echo ====================================
echo   ✅ 构建成功！
echo ====================================
echo 输出位置: dist\neoray-server-windows.zip
echo 可执行文件: build\neoray-server.exe
echo.
echo 运行方式:
echo   cd build
echo   neoray-server.exe
echo.
echo 或仅服务器模式:
echo   neoray-server.exe --no-tui
echo.
pause
