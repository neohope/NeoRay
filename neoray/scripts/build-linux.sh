#!/bin/bash

echo "===================================="
echo "  NeoRay 后端 Linux 构建脚本"
echo "===================================="
echo ""

cd "$(dirname "$0")/.."

echo "[1/6] 清理构建缓存..."
rm -rf build dist

echo "[2/6] 检查 Go 环境..."
if ! command -v go &> /dev/null; then
    echo "❌ 未找到 Go，请先安装 Go"
    echo "下载地址: https://golang.org/dl/"
    exit 1
fi
echo "✅ Go 已安装"
go version

echo ""
echo "[3/6] 获取依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "❌ 依赖获取失败！"
    exit 1
fi
echo "✅ 依赖已获取"

echo ""
echo "[4/6] 运行测试..."
go test ./... -v
if [ $? -ne 0 ]; then
    echo "⚠️  测试失败，但继续构建..."
else
    echo "✅ 测试通过"
fi

echo ""
echo "[5/6] 构建 Linux 可执行文件..."
mkdir -p build
export GOOS=linux
export GOARCH=amd64
go build -ldflags "-s -w" -o build/neoray-server ./cmd/server
if [ $? -ne 0 ]; then
    echo "❌ 构建失败！"
    exit 1
fi
chmod +x build/neoray-server
echo "✅ 构建成功"

echo ""
echo "[6/6] 打包..."
mkdir -p dist

# 复制配置文件
cp config.yaml.example build/config.yaml.example 2>/dev/null || true
cp config.yaml build/config.yaml 2>/dev/null || cp config.yaml.example build/config.yaml

# 创建 tar.gz 包
cd build
tar -czf ../dist/neoray-server-linux.tar.gz .
cd ..

echo ""
echo "===================================="
echo "  ✅ 构建成功！"
echo "===================================="
echo "输出位置: dist/neoray-server-linux.tar.gz"
echo "可执行文件: build/neoray-server"
echo ""
echo "运行方式:"
echo "  cd build"
echo "  ./neoray-server"
echo ""
echo "或仅服务器模式:"
echo "  ./neoray-server --no-tui"
echo ""
