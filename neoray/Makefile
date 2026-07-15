.PHONY: help build-windows build-macos build-linux clean run test install

# 帮助信息
help:
	@echo "NeoRay 后端构建工具"
	@echo ""
	@echo "可用命令："
	@echo "  make              - 显示此帮助"
	@echo "  make all          - 构建所有平台"
	@echo "  make windows      - 构建 Windows 版本"
	@echo "  make macos        - 构建 macOS 版本"
	@echo "  make linux        - 构建 Linux 版本"
	@echo "  make test         - 运行测试"
	@echo "  make clean        - 清理构建缓存"
	@echo "  make run          - 运行开发版本"
	@echo "  make install      - 安装到系统"
	@echo "  make fmt          - 格式化代码"

# 构建所有平台
all: windows macos linux

# Windows 构建
windows:
	@echo "构建 Windows 版本..."
	@if [ "$(shell uname -s)" = "Windows_NT" ]; then \
		./scripts/build-windows.bat; \
	else \
		echo "请在 Windows 上运行此命令，或使用交叉编译"; \
		GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o build/neoray.exe ./cmd/server; \
	fi

# macOS 构建
macos:
	@echo "构建 macOS 版本..."
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		chmod +x ./scripts/build-macos.sh; \
		./scripts/build-macos.sh; \
	else \
		echo "请在 macOS 上运行此命令，或使用交叉编译"; \
		GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o build/neoray ./cmd/server; \
	fi

# Linux 构建
linux:
	@echo "构建 Linux 版本..."
	@if [ "$(shell uname -s)" = "Linux" ]; then \
		chmod +x ./scripts/build-linux.sh; \
		./scripts/build-linux.sh; \
	else \
		echo "请在 Linux 上运行此命令，或使用交叉编译"; \
		GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o build/neoray ./cmd/server; \
	fi

# 运行测试
test:
	@echo "运行测试..."
	go test ./... -v

# 清理
clean:
	@echo "清理..."
	rm -rf build
	rm -rf dist
	go clean

# 运行
run:
	@echo "运行开发版本..."
	go run ./cmd/server

# 安装
install:
	@echo "安装到系统..."
	go install ./cmd/server

# 格式化
fmt:
	@echo "格式化代码..."
	go fmt ./...
