.PHONY: help all windows macos linux clean run test install install-skills fmt

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
	@echo "  make install      - 安装到系统（含内置技能）"
	@echo "  make install-skills - 只同步内置技能到 ~/.neoray/skills"
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

# 技能安装目录（与服务端 config.HomeDir 一致，可用 NEORAY_SKILLS_DIR 覆盖）
NEORAY_HOME ?= $(HOME)/.neoray
SKILLS_DIR ?= $(NEORAY_SKILLS_DIR)
ifeq ($(strip $(SKILLS_DIR)),)
SKILLS_DIR := $(NEORAY_HOME)/skills
endif

# 安装目录：默认 ~/.local/bin（与 launchd 服务里 ProgramArguments 一致），
# 可用 BIN_DIR 覆盖。这里不用 go install，因为它会把二进制放到 GOPATH/bin，
# 与服务实际启动的路径不是同一个地方，升级后容易"装了但没生效"。
BIN_DIR ?= $(HOME)/.local/bin

# 安装
install: install-skills
	@echo "构建并安装二进制到 $(BIN_DIR)..."
	@mkdir -p "$(BIN_DIR)"
	go build -trimpath -ldflags "-s -w" -o "$(BIN_DIR)/neoray" ./cmd/server
	@echo "✅ 已安装 $(BIN_DIR)/neoray；技能位于 $(SKILLS_DIR)"
	@echo "提示：重启服务生效 —— launchctl kickstart -k gui/$$(id -u)/ai.neoray.gateway"

# 同步内置技能：服务端默认从 <HomeDir>/skills 读取，
# 不拷贝的话运行时会加载到 0 个技能（agent 会答不出自己有哪些技能）
install-skills:
	@echo "同步内置技能到 $(SKILLS_DIR)..."
	@mkdir -p "$(SKILLS_DIR)"
	@cp -R skills/. "$(SKILLS_DIR)/"
	@echo "✅ 已同步 $$(ls -d "$(SKILLS_DIR)"/*/ 2>/dev/null | wc -l | tr -d ' ') 个技能"

# 格式化
fmt:
	@echo "格式化代码..."
	go fmt ./...
