.PHONY: help build-windows build-macos build-linux build-web clean run

# 帮助信息
help:
	@echo "NeoRay 构建工具"
	@echo ""
	@echo "可用命令："
	@echo "  make              - 显示此帮助"
	@echo "  make all          - 构建所有平台"
	@echo "  make windows      - 构建 Windows 版本"
	@echo "  make macos        - 构建 macOS 版本"
	@echo "  make linux        - 构建 Linux 版本"
	@echo "  make web          - 构建 Web 版本"
	@echo "  make generate     - 生成代码（Freezed/JSON/Hive）"
	@echo "  make clean        - 清理构建缓存"
	@echo "  make run          - 运行开发版本"
	@echo "  make run-web      - 运行 Web 开发版本"
	@echo "  make format       - 格式化代码"

# 构建所有平台
all: windows macos linux web

# Windows 构建
windows:
	@echo "构建 Windows 版本..."
	@if [ "$(shell uname -s)" = "Windows_NT" ]; then \
		./scripts/build-windows.bat; \
	else \
		echo "请在 Windows 上运行此命令"; \
	fi

# macOS 构建
macos:
	@echo "构建 macOS 版本..."
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		chmod +x ./scripts/build-macos.sh; \
		./scripts/build-macos.sh; \
	else \
		echo "请在 macOS 上运行此命令"; \
	fi

# Linux 构建
linux:
	@echo "构建 Linux 版本..."
	@if [ "$(shell uname -s)" = "Linux" ]; then \
		chmod +x ./scripts/build-linux.sh; \
		./scripts/build-linux.sh; \
	else \
		echo "请在 Linux 上运行此命令"; \
	fi

# Web 构建
web:
	@echo "构建 Web 版本..."
	@if [ "$(shell uname -s)" = "Windows_NT" ]; then \
		./scripts/build-web.bat; \
	else \
		chmod +x ./scripts/build-web.sh; \
		./scripts/build-web.sh; \
	fi

# 运行 Web 开发版本
run-web:
	@echo "运行 Web 开发版本..."
	flutter run -d chrome

# 代码生成
generate:
	@echo "生成代码..."
	flutter pub run build_runner build --delete-conflicting-outputs

# 清理
clean:
	@echo "清理..."
	flutter clean
	rm -rf .dart_tool
	rm -rf build

# 运行
run:
	@echo "运行开发版本..."
	flutter run

# 格式化
format:
	@echo "格式化代码..."
	flutter format lib/
