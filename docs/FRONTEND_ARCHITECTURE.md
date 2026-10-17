
# NeoRay 前端架构文档

## 目标平台

- ✅ **桌面端**: Windows, macOS, Linux
- ✅ **Web 端**: 现代浏览器
- ❌ ~~移动端 (iOS/Android)~~ (暂时不包含)

---

## 技术栈

| 分类 | 技术选型 |
|------|----------|
| 框架 | Flutter 3.x |
| 状态管理 | Riverpod 2.x |
| UI 组件 | Material 3 (桌面优化) |
| 路由 | GoRouter |
| WebSocket | web_socket_channel |
| Markdown | flutter_markdown + flutter_highlight |
| 窗口管理 | window_manager |
| 文件选择 | file_picker |
| 系统托盘 | system_tray (桌面端) |
| 本地存储 | shared_preferences + sqflite (桌面) |

---

## 整体架构 (Clean Architecture + Riverpod)

```
┌─────────────────────────────────────────────────────────────┐
│                    Presentation Layer                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Pages      │  │   Widgets    │  │  Overlays    │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  State Management (Riverpod)                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Notifiers  │  │   Providers  │  │   Listeners  │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Use Cases / Services                      │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        Data Layer                            │
│  ┌──────────────────┐         ┌──────────────────┐         │
│  │   Repositories   │◀───────▶│    Data Sources  │         │
│  └──────────────────┘         └──────────────────┘         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Desktop / Web Infrastructure                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ Window   │ │  Tray    │ │  Storage │ │ Platform │       │
│  │  Mgmt    │ │          │ │          │ │  Utils   │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
└─────────────────────────────────────────────────────────────┘
```

---

## 目录结构

```
neoray_app/                         # Flutter 客户端（项目名: neoray_app）
├── lib/
│   ├── main.dart                   # 应用入口（ProviderScope + 全局 WS 事件分发）
│   │
│   ├── constants/                  # 常量与尺寸
│   │   ├── app_defaults.dart       # 默认值（serverUrl / 主题 / 模型参数）
│   │   ├── app_dimensions.dart     # 间距、圆角、尺寸
│   │   ├── app_strings.dart        # 文案
│   │   ├── app_timings.dart        # 超时、动画时长
│   │   └── constants.dart          # 汇总导出
│   │
│   ├── models/                     # 数据模型（freezed 生成不可变类）
│   │   ├── app_config.dart         # AppConfig（客户端本地）+ ServerConfig（服务端）
│   │   ├── message.dart            # 消息实体
│   │   └── session.dart            # 会话实体
│   │
│   ├── pages/                      # 页面
│   │   ├── chat_page.dart          # 聊天页（会话列表 + 消息区 + 输入区）
│   │   └── config_page.dart        # 配置页（LLM / Channel / Tools）
│   │
│   ├── providers/
│   │   └── providers.dart          # 全部 Riverpod 状态（配置/会话/流式 UI）
│   │
│   ├── services/                   # 通信层
│   │   ├── api_service.dart        # REST 客户端（含 X-Api-Key 认证头）
│   │   └── websocket_service.dart  # WebSocket（指数退避重连、事件解析）
│   │
│   ├── theme/
│   │   └── app_theme.dart          # 亮/暗主题
│   │
│   ├── utils/
│   │   └── logger.dart             # 日志封装
│   │
│   └── widgets/                    # 组件
│       ├── message_bubble.dart     # 消息气泡
│       └── sidebar.dart            # 会话侧边栏
│
├── macos/                          # macOS 平台工程
├── windows/                        # Windows 平台工程
├── web/                            # Web 平台脚手架（未构建，实际 Web UI 见 webui/）
└── scripts/                        # 构建脚本
```

> 说明：仓库根目录的 `webui/` 是 **Web UI 前端源码**（React + Vite），
> 与本目录（Flutter 客户端）是两套独立前端；Flutter 负责桌面/移动端。
> 早期文档中的 `neorayui/` 为历史命名，现已更名为 `neoray_app/`。

---

## 桌面端特有功能

### 窗口管理
- 记忆窗口大小和位置
- 最小化到系统托盘
- 置顶选项
- 多窗口支持 (可选)

### 系统托盘
- 托盘图标
- 右键菜单 (显示/隐藏/退出)
- 新消息通知 (闪烁/气泡)

### 文件集成
- 拖拽文件到窗口上传
- 文件选择器支持
- 打开文件所在位置

### 快捷键
- `Ctrl/Cmd + N` - 新建会话
- `Ctrl/Cmd + W` - 关闭当前会话
- `Ctrl/Cmd + ,` - 打开设置
- `Ctrl/Cmd + K` - 搜索会话

### 菜单条
- 文件菜单 (新建/打开/导出)
- 编辑菜单 (复制/粘贴/清空)
- 视图菜单 (主题/缩放)
- 帮助菜单 (关于/检查更新)

---

## Web 端特有功能

- PWA 支持 (可安装到桌面)
- 响应式布局适配各种屏幕
- 分享链接
- 导出会话为 JSON/Markdown

---

## 响应式布局设计

```
┌─────────────────────────────────────────────────────────────┐
│  Desktop Layout (宽屏)                                       │
│  ┌──────────────┬─────────────────────────────────────────┐  │
│  │   Sidebar    │            Main Content                 │  │
│  │  - Sessions  │                                         │  │
│  │  - New Chat  │            Chat Area                    │  │
│  │  - Settings  │                                         │  │
│  │              │                                         │  │
│  │              │                                         │  │
│  └──────────────┴─────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  Tablet / Narrow Layout                                      │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │  [Menu] 会话标题                        ⋮              │ │
│  ├─────────────────────────────────────────────────────────┤ │
│  │                                                         │ │
│  │                    Chat Area                            │ │
│  │                                                         │ │
│  │                                                         │ │
│  ├─────────────────────────────────────────────────────────┤ │
│  │  [Input Field]                                     [Send]│ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## 核心状态管理 (Riverpod)

```dart
// presentation/providers/session_provider.dart
final sessionListProvider = StateNotifierProvider&lt;SessionNotifier, AsyncValue&lt;List&lt;Session&gt;&gt;&gt;((ref) {
  final repo = ref.watch(sessionRepositoryProvider);
  return SessionNotifier(repo);
});

final currentSessionIdProvider = StateProvider&lt;String?&gt;((ref) =&gt; null);

// presentation/providers/message_provider.dart
final messageProvider = StateNotifierProvider.family&lt;MessageNotifier, AsyncValue&lt;List&lt;Message&gt;&gt;, String&gt;((ref, sessionId) {
  final repo = ref.watch(messageRepositoryProvider);
  return MessageNotifier(repo, sessionId);
});

// presentation/providers/connection_provider.dart
enum ConnectionStatus { disconnected, connecting, connected, error }
final connectionStatusProvider = StateProvider&lt;ConnectionStatus&gt;((ref) =&gt; ConnectionStatus.disconnected);

// presentation/providers/theme_provider.dart
final themeModeProvider = StateProvider&lt;ThemeMode&gt;((ref) =&gt; ThemeMode.system);
```

---

## 核心服务

```dart
// services/window_service.dart (桌面端)
class WindowService {
  Future&lt;void&gt; init();
  Future&lt;void&gt; setWindowSize(Size size);
  Future&lt;void&gt; setMinimumSize(Size size);
  Future&lt;void&gt; center();
  Future&lt;void&gt; hide();
  Future&lt;void&gt; show();
}

// services/tray_service.dart (桌面端)
class TrayService {
  Future&lt;void&gt; init();
  Future&lt;void&gt; show();
  Future&lt;void&gt; setIcon(String path);
  Future&lt;void&gt; showMessage(String title, String message);
}

// services/websocket_service.dart
class WebSocketService {
  Future&lt;void&gt; connect(String url);
  Future&lt;void&gt; disconnect();
  void send(Message message);
  Stream&lt;Message&gt; get messages;
  Stream&lt;ConnectionStatus&gt; get status;
}
```

---

## 页面导航 (GoRouter)

```dart
final router = GoRouter(
  routes: [
    GoRoute(path: '/', builder: (context, state) =&gt; const SplashPage()),
    GoRoute(path: '/welcome', builder: (context, state) =&gt; const WelcomePage()),
    GoRoute(path: '/home', builder: (context, state) =&gt; const HomePage()),
    GoRoute(path: '/chat/:id', builder: (context, state) =&gt; ChatPage(sessionId: state.pathParameters['id']!)),
    GoRoute(path: '/settings', builder: (context, state) =&gt; const SettingsPage()),
  ],
);
```

---

## 依赖包列表

```yaml
dependencies:
  flutter:
    sdk: flutter

  # 状态管理
  flutter_riverpod: ^2.3.0
  riverpod_annotation: ^2.1.0

  # 路由
  go_router: ^10.0.0

  # UI 组件
  material_color_utilities: ^0.8.0
  flutter_svg: ^2.0.0

  # Markdown
  flutter_markdown: ^0.6.0
  flutter_highlight: ^0.7.0
  markdown: ^7.0.0

  # WebSocket
  web_socket_channel: ^2.4.0

  # 网络
  http: ^1.1.0

  # 本地存储
  shared_preferences: ^2.2.0
  sqflite: ^2.3.0
  path_provider: ^2.1.0

  # 文件操作
  file_picker: ^6.1.0

  # 桌面端特定
  window_manager: ^0.3.0
  system_tray: ^2.0.0
  desktop_multi_window: ^0.2.0 (可选)
  shortcut_menu: ^0.1.0 (可选)

  # 工具类
  freezed_annotation: ^2.4.0
  json_annotation: ^4.8.0
  uuid: ^4.2.0
  intl: ^0.18.0

dev_dependencies:
  build_runner: ^2.4.0
  freezed: ^2.4.0
  json_serializable: ^6.7.0
  riverpod_generator: ^2.3.0
  flutter_test:
    sdk: flutter
```

---

## 实现优先级

### Phase 1: 核心界面 (Week 9)
- [ ] 项目初始化和配置
- [ ] 基础布局 (桌面分栏 + Web 响应式)
- [ ] 会话列表页
- [ ] 聊天页面 (基础)

### Phase 2: 连接和消息 (Week 10)
- [ ] WebSocket 服务集成
- [ ] 消息收发
- [ ] Markdown 渲染
- [ ] 代码高亮

### Phase 3: 桌面端增强 (Week 11)
- [ ] 窗口管理
- [ ] 系统托盘
- [ ] 快捷键
- [ ] 菜单条

### Phase 4:  polish (Week 12)
- [ ] 主题切换
- [ ] 设置页面
- [ ] Web PWA 支持
- [ ] 测试和优化

