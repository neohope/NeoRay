.PHONY: help all backend frontend webui web windows macos linux clean

# 当前平台对应的子 Makefile 目标
ifeq ($(shell uname -s),Darwin)
PLATFORM_TARGET := macos
else ifeq ($(shell uname -s),Linux)
PLATFORM_TARGET := linux
else
PLATFORM_TARGET := windows
endif

# 帮助信息
help:
	@echo "NeoRay 完整构建工具"
	@echo ""
	@echo "可用命令："
	@echo "  make              - 显示此帮助"
	@echo "  make all          - 构建全部（后端 + 桌面客户端，当前平台）"
	@echo "  make backend      - 仅构建后端（自动内嵌 Web UI 资源）"
	@echo "  make webui        - 仅构建 Web UI 前端资源（React + Vite）"
	@echo "  make frontend     - 仅构建桌面客户端（Flutter）"
	@echo "  make windows      - 构建 Windows 版本"
	@echo "  make macos        - 构建 macOS 版本"
	@echo "  make linux        - 构建 Linux 版本"
	@echo "  make clean        - 清理所有构建"
	@echo ""
	@echo "说明：Web UI（webui/，React）的构建产物由后端 go:embed 内嵌进二进制，"
	@echo "      因此 backend / macos / windows / linux 均已自动包含 Web UI 构建。"
	@echo "      历史上曾用 Flutter 构建 Web 产物（neoray_app/web），现已不再采用。"

# 构建全部（检测当前平台）
all:
	@echo "检测平台..."
	@if [ "$(shell uname -s)" = "Windows_NT" ]; then \
		$(MAKE) windows; \
	elif [ "$(shell uname -s)" = "Darwin" ]; then \
		$(MAKE) macos; \
	elif [ "$(shell uname -s)" = "Linux" ]; then \
		$(MAKE) linux; \
	else \
		echo "未知平台"; \
	fi

# Windows 完整构建（后端含内嵌 Web UI + 桌面客户端）
windows:
	@echo "构建 Windows 版本..."
	@if [ "$(shell uname -s)" = "Windows_NT" ]; then \
		./scripts/build-all-windows.bat; \
	else \
		echo "请在 Windows 上运行此命令"; \
	fi

# macOS 完整构建
macos:
	@echo "构建 macOS 版本..."
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		chmod +x ./scripts/build-all-macos.sh; \
		./scripts/build-all-macos.sh; \
	else \
		echo "请在 macOS 上运行此命令"; \
	fi

# Linux 完整构建
linux:
	@echo "构建 Linux 版本..."
	@if [ "$(shell uname -s)" = "Linux" ]; then \
		chmod +x ./scripts/build-all-linux.sh; \
		./scripts/build-all-linux.sh; \
	else \
		echo "请在 Linux 上运行此命令"; \
	fi

# 仅构建后端（其构建脚本会自动先构建 Web UI 资源并内嵌）
backend:
	@echo "构建后端（含内嵌 Web UI）..."
	@cd neoray && $(MAKE) $(PLATFORM_TARGET)

# 仅构建 Web UI 前端资源（React + Vite，产物由后端 go:embed 使用）
webui:
	@echo "构建 Web UI 前端资源..."
	@cd webui && npm install && npm run build

# web 为 webui 的别名，保持历史命令可用
web: webui

# 仅构建桌面客户端（Flutter）
frontend:
	@echo "构建桌面客户端..."
	@cd neoray_app && $(MAKE) $(PLATFORM_TARGET)

# 清理
clean:
	@echo "清理..."
	@cd neoray && $(MAKE) clean
	@cd neoray_app && $(MAKE) clean
