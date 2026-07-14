import 'dart:async';
import 'dart:io';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:toml/toml.dart';
import 'package:uuid/uuid.dart';
import '../models/session.dart';
import '../models/message.dart';
import '../models/app_config.dart';
import '../services/api_service.dart';
import '../services/websocket_service.dart';
import '../constants/constants.dart';
import '../utils/logger.dart';

const _uuid = Uuid();

// Channel ID Provider - 运行时生成唯一 ID，生产环境应替换为认证系统提供的标识
final channelIdProvider = StateProvider<String>((ref) => _uuid.v4());

// User ID Provider - 运行时生成唯一 ID，生产环境应替换为认证系统提供的标识
final userIdProvider = StateProvider<String>((ref) => _uuid.v4());

// API Service Provider - 首先获取配置中的服务器地址
final apiServiceProvider = Provider<ApiService>((ref) {
  final channelId = ref.watch(channelIdProvider);
  final userId = ref.watch(userIdProvider);
  final config = ref.watch(appConfigProvider);
  return ApiService(
    baseUrl: config.serverUrl,
    channelId: channelId,
    userId: userId,
  );
});

// Global Error Provider
final globalErrorProvider = StateProvider<String?>((ref) => null);

// WebSocket Service Provider
final webSocketServiceProvider = Provider<WebSocketService>((ref) {
  final service = WebSocketService();
  ref.onDispose(() => service.dispose());
  return service;
});

// ─── 客户端本地配置（serverUrl + themeMode） ───

final appConfigProvider = StateNotifierProvider<AppConfigNotifier, AppConfig>((ref) {
  return AppConfigNotifier();
});

class AppConfigNotifier extends StateNotifier<AppConfig> {
  static const _configFileName = 'client.toml';

  bool _loaded = false;
  bool get loaded => _loaded;

  AppConfigNotifier() : super(const AppConfig()) {
    _load();
  }

  Future<File> _getConfigFile() async {
    final dir = await getApplicationDocumentsDirectory();
    return File('${dir.path}/$_configFileName');
  }

  Future<void> _load() async {
    try {
      final file = await _getConfigFile();
      if (await file.exists()) {
        final tomlStr = await file.readAsString();
        final map = TomlDocument.parse(tomlStr).toMap();
        state = AppConfig.fromJson(map);
      }
    } catch (e) {
      logger.e('加载配置失败', error: e);
    } finally {
      _loaded = true;
    }
  }

  Future<void> persist() async {
    try {
      final file = await _getConfigFile();
      final tomlStr = TomlDocument.fromMap(state.toJson()).toString();
      await file.writeAsString(tomlStr);
    } catch (e) {
      logger.e('保存配置失败', error: e);
    }
  }

  void updateServerUrl(String url) {
    state = state.copyWith(serverUrl: url);
  }

  void updateThemeMode(String mode) {
    state = state.copyWith(themeMode: mode);
    persist();
  }
}

// ─── 服务端配置（通过 API 读写） ───

final serverConfigProvider = StateNotifierProvider<ServerConfigNotifier, AsyncValue<ServerConfig>>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return ServerConfigNotifier(apiService);
});

class ServerConfigNotifier extends StateNotifier<AsyncValue<ServerConfig>> {
  final ApiService _apiService;

  ServerConfigNotifier(this._apiService) : super(const AsyncValue.loading()) {
    load();
  }

  Future<void> load() async {
    state = const AsyncValue.loading();
    try {
      final config = await _apiService.getServerConfig();
      state = AsyncValue.data(config);
    } catch (e, stackTrace) {
      state = AsyncValue.error(e, stackTrace);
    }
  }

  Future<void> updateLLMProvider(String providerName, Map<String, dynamic> updates) async {
    try {
      final config = await _apiService.updateServerConfig({
        'llm': {
          'providers': {
            providerName: updates,
          },
        },
      });
      state = AsyncValue.data(config);
    } catch (e, stackTrace) {
      logger.e('更新 LLM 配置失败', error: e, stackTrace: stackTrace);
      rethrow;
    }
  }

  Future<void> updateChannels(Map<String, dynamic> updates) async {
    try {
      final config = await _apiService.updateServerConfig({
        'channels': updates,
      });
      state = AsyncValue.data(config);
    } catch (e, stackTrace) {
      logger.e('更新 Channel 配置失败', error: e, stackTrace: stackTrace);
      rethrow;
    }
  }

  Future<void> updateTools(Map<String, dynamic> updates) async {
    try {
      final config = await _apiService.updateServerConfig({
        'tools': updates,
      });
      state = AsyncValue.data(config);
    } catch (e, stackTrace) {
      logger.e('更新工具配置失败', error: e, stackTrace: stackTrace);
      rethrow;
    }
  }
}

// Session List Provider
final sessionListProvider = StateNotifierProvider<SessionListNotifier, AsyncValue<List<Session>>>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  return SessionListNotifier(apiService, ref);
});

class SessionListNotifier extends StateNotifier<AsyncValue<List<Session>>> {
  final ApiService _apiService;
  final Ref _ref;

  SessionListNotifier(this._apiService, this._ref) : super(const AsyncValue.loading()) {
    loadSessions();
  }

  Future<void> loadSessions() async {
    state = const AsyncValue.loading();
    try {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      final sessions = await _apiService.getSessions(channelId: channelId, userId: userId);
      state = AsyncValue.data(sessions);
    } catch (e, stackTrace) {
      state = AsyncValue.error(e, stackTrace);
    }
  }

  Future<void> createSession({String? title}) async {
    try {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      await _apiService.createSession(channelId: channelId, userId: userId, title: title);
      await loadSessions();
    } catch (e, stackTrace) {
      state = AsyncValue.error(e, stackTrace);
    }
  }

  Future<void> deleteSession(String sessionId) async {
    try {
      await _apiService.deleteSession(sessionId);
      await loadSessions();
    } catch (e, stackTrace) {
      state = AsyncValue.error(e, stackTrace);
    }
  }
}

// Current Session Provider
final currentSessionProvider = StateNotifierProvider<CurrentSessionNotifier, Session?>((ref) {
  final apiService = ref.watch(apiServiceProvider);
  final webSocket = ref.watch(webSocketServiceProvider);
  return CurrentSessionNotifier(apiService, webSocket, ref);
});

class CurrentSessionNotifier extends StateNotifier<Session?> {
  final ApiService _apiService;
  final WebSocketService _webSocketService;
  final Ref _ref;

  CurrentSessionNotifier(this._apiService, this._webSocketService, this._ref) : super(null);

  Future<void> selectSession(String sessionId) async {
    try {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      final session = await _apiService.getSession(sessionId, channelId: channelId, userId: userId);
      state = session;
      _webSocketService.joinSession(sessionId, channelId: channelId, userId: userId);
    } catch (e, stackTrace) {
      logger.e('选择会话失败', error: e, stackTrace: stackTrace);
      // 保持当前状态不变，错误由调用方处理
      rethrow;
    }
  }

  Future<void> newSession({String? title}) async {
    try {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      final session = Session.create(channelId: channelId, userId: userId, title: title);
      state = session;
    } catch (e, stackTrace) {
      logger.e('创建新会话失败', error: e, stackTrace: stackTrace);
      rethrow;
    }
  }

  Future<void> sendMessage(String message) async {
    final session = state;
    if (session == null) return;

    final channelId = _ref.read(channelIdProvider);
    final userId = _ref.read(userIdProvider);
    final userMessage = Message.user(message, channelId: channelId, userId: userId, sessionId: session.id);
    state = session.copyWith(messages: [...session.messages, userMessage]);

    if (_webSocketService.isConnected) {
      _webSocketService.sendChat(
        message: message,
        sessionId: session.id,
        channelId: channelId,
        userId: userId,
        stream: true,
      );
    } else {
      try {
        final response = await _apiService.sendMessage(
          sessionId: session.id,
          channelId: channelId,
          userId: userId,
          message: message,
        );
        final current = state;
        if (current == null) return;
        state = current.copyWith(messages: [...current.messages, response]);
      } catch (e, stackTrace) {
        logger.e('发送消息失败', error: e, stackTrace: stackTrace);
        final current = state;
        if (current == null) return;
        final userMsgTime = userMessage.timestamp;
        state = current.copyWith(
          messages: current.messages.where((m) => m.timestamp != userMsgTime).toList(),
        );
        rethrow;
      }
    }
  }

  // 流式拼接用 StringBuffer 缓冲，O(n²)→O(n) 字符串拼接
  // 流式期间不更新 messages 列表（UI 从 currentStreamingContentProvider 渲染），
  // 仅在流式结束时一次性追加最终消息，避免每 chunk 都 O(n) 拷贝列表
  final StringBuffer _streamBuffer = StringBuffer();
  final StringBuffer _reasoningBuffer = StringBuffer();

  void addStreamingChunk(String content) {
    final session = state;
    if (session == null) return;
    // 仅缓冲内容，不更新 messages 列表 — O(1)
    _streamBuffer.write(content);
  }

  void addReasoningChunk(String content) {
    final session = state;
    if (session == null) return;
    // 仅缓冲内容，不更新 messages 列表 — O(1)
    _reasoningBuffer.write(content);
  }

  /// 流式结束时将最终消息追加到列表 — 仅调用一次，O(n) 但 n=1
  void completeStreaming() {
    final session = state;
    if (session == null) return;

    final content = _streamBuffer.toString();
    _streamBuffer.clear();

    if (content.isNotEmpty) {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      state = session.copyWith(
        messages: [...session.messages, Message.assistant(content, null, channelId, userId, session.id)],
      );
    }
  }

  void completeReasoning() {
    final session = state;
    if (session == null) return;

    final reasoningContent = _reasoningBuffer.toString();
    _reasoningBuffer.clear();

    if (reasoningContent.isNotEmpty) {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      state = session.copyWith(
        messages: [...session.messages, Message.assistant('', null, channelId, userId, session.id, reasoningContent, true)],
      );
    }
  }

  void addMessage(Message message) {
    if (state == null) return;
    state = state!.copyWith(messages: [...state!.messages, message]);
  }

  void clearSession() {
    state = null;
  }

  void renameTitle(String newTitle) {
    final session = state;
    if (session == null) return;
    state = session.copyWith(title: newTitle);
  }
}

// Chat Streaming State
final chatStreamingProvider = StateProvider<bool>((ref) => false);
final currentStreamingContentProvider = StateProvider<String>((ref) => '');

// Reasoning Streaming State
final reasoningStreamingProvider = StateProvider<bool>((ref) => false);
final currentReasoningContentProvider = StateProvider<String>((ref) => '');

// UI State Providers
final activePageProvider = StateProvider<AppPage>((ref) => AppPage.chat);
final isSettingsVisibleProvider = StateProvider<bool>((ref) => false);

enum AppPage {
  chat,
  config,
}
