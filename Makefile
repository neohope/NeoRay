.PHONY: help all backend frontend windows macos linux web clean

# 帮助信息
help:
	@echo "NeoRay 完整构建工具"
	@echo ""
	@echo "可用命令："
	@echo "  make              - 显示此帮助"
	@echo "  make all          - 构建全部（后端 + 前端桌面）"
	@echo "  make all-web      - 构建全部（后端 + 前端 Web）"
	@echo "  make backend      - 仅构建后端"
	@echo "  make frontend     - 仅构建前端桌面"
	@echo "  make frontend-web - 仅构建前端 Web"
	@echo "  make windows      - 构建 Windows 版本"
	@echo "  make macos        - 构建 macOS 版本"
	@echo "  make linux        - 构建 Linux 版本"
	@echo "  make web          - 构建 Web 版本"
	@echo "  make clean        - 清理所有构建"

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

# Windows 完整构建
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

# 仅构建后端
backend:
	@echo "构建后端..."
	@cd neoray && $(MAKE)

# 仅构建前端
frontend:
	@echo "构建前端..."
	@cd neoray_app && $(MAKE)

# 构建 Web 版本
web:
	@echo "构建 Web 版本..."
	@cd neoray_app && $(MAKE) web

# 构建全部（后端 + Web 前端）
all-web: backend web
	@echo "✅ Web 版本构建完成！"
	@echo "后端: neoray/build/"
	@echo "前端: neoray_app/build/web/"

# 仅构建前端 Web
frontend-web:
	@echo "构建前端 Web..."
	@cd neoray_app && $(MAKE) web

# 清理
clean:
	@echo "清理..."
	@cd neoray && $(MAKE) clean
	@cd neoray_app && $(MAKE) clean
