# NeoRay Flutter App

基于 Flutter 的 NeoRay AI Agent 桌面应用。

## 功能特性

- 🤖 AI 聊天 - 与 AI 助手进行对话
- 💬 流式响应 - 实时流式显示 AI 回复
- 🛠️ 工具调用 - 支持多种 AI 工具调用（文件、Shell 等）
- 📱 响应式 UI - 现代化的聊天界面
- 🔧 配置管理 - 灵活的模型和工具配置
- 🌐 WebSocket 连接 - 实时双向通信

## 项目结构

```
lib/
├── main.dart                    # 应用入口
├── models/                      # 数据模型
│   ├── session.dart           # 会话模型
│   ├── message.dart         # 消息模型
│   └── app_config.dart    # 应用配置模型
├── pages/                      # 页面组件
│   ├── chat_page.dart       # 聊天页面
│   └── config_page.dart    # 配置页面
├── widgets/                    # UI组件
│   ├── sidebar.dart         # 侧边栏组件
│   └── message_bubble.dart # 消息气泡组件
├── providers/                  # 状态管理
│   └── providers.dart     # Riverpod 提供者
├── services/                 # 服务层
│   ├── api_service.dart   # REST API 服务
│   └── websocket_service.dart # WebSocket 服务
└── theme/                   # 主题配置
    └── app_theme.dart     # 应用主题
```

## 快速开始

### 前置要求

- Flutter 3.0+
- Dart 3.0+
- 已运行的 NeoRay 后端服务

### 运行

1. 确保后端服务运行在 `http://localhost:8080`
2. 运行以下命令：

```bash
cd neoray_app
flutter pub get
flutter run
```

### 生成代码

如果需要重新生成 Freezed/JSON 代码：

```bash
flutter pub run build_runner watch --delete-conflicting-outputs
```

## 技术栈

- **Flutter** - 跨平台 UI 框架
- **Riverpod** - 状态管理
- **Freezed** - 数据类生成
- **Hive** - 本地存储
- **http** - HTTP 请求
- **web_socket_channel** - WebSocket 通信

## 注意事项

⚠️ **重要**：根据用户要求，本应用**不会**直接修改本地文件，所有文件操作通过后端 API 完成。
