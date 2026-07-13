import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import '../models/session.dart';
import '../models/message.dart';
import '../models/app_config.dart';
import '../services/api_service.dart';
import '../services/websocket_service.dart';
import '../constants/constants.dart';
import '../utils/logger.dart';

// Channel ID Provider
final channelIdProvider = StateProvider<String>((ref) => AppStrings.defaultChannelId);

// User ID Provider
final userIdProvider = StateProvider<String>((ref) => AppStrings.defaultUserId);

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

// App Config Provider
final appConfigProvider = StateNotifierProvider<AppConfigNotifier, AppConfig>((ref) {
  return AppConfigNotifier();
});

class AppConfigNotifier extends StateNotifier<AppConfig> {
  static const _configFileName = 'neoray_config.json';

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
        final json = await file.readAsString();
        state = AppConfig.fromJson(jsonDecode(json) as Map<String, dynamic>);
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
      await file.writeAsString(jsonEncode(state.toJson()));
    } catch (e) {
      logger.e('保存配置失败', error: e);
    }
  }

  Timer? _persistDebounce;

  void updateConfig(AppConfig config) {
    state = config;
    _debouncePersist();
  }

  void updateLLMConfig(LLMConfig config) {
    state = state.copyWith(llm: config);
    _debouncePersist();
  }

  void updateChannelConfig(ChannelConfig config) {
    state = state.copyWith(channel: config);
    _debouncePersist();
  }

  void updateToolConfig(ToolConfig config) {
    state = state.copyWith(tools: config);
    _debouncePersist();
  }

  void _debouncePersist() {
    _persistDebounce?.cancel();
    _persistDebounce = Timer(const Duration(seconds: 2), persist);
  }

  @override
  void dispose() {
    _persistDebounce?.cancel();
    super.dispose();
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

  // 流式拼接用 StringBuffer 缓冲，避免 O(n²) 字符串拼接
  final StringBuffer _streamBuffer = StringBuffer();
  final StringBuffer _reasoningBuffer = StringBuffer();

  void addStreamingChunk(String content) {
    final session = state;
    if (session == null) return;

    _streamBuffer.write(content);

    final messages = session.messages;
    if (messages.isNotEmpty && messages.last.role == 'assistant') {
      // 只替换最后一条消息，避免拷贝整个列表
      final updated = List<Message>.from(messages);
      updated[updated.length - 1] = messages.last.copyWith(
        content: _streamBuffer.toString(),
      );
      state = session.copyWith(messages: updated);
    } else {
      _streamBuffer.clear();
      _streamBuffer.write(content);
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      state = session.copyWith(
        messages: [...messages, Message.assistant(content, null, channelId, userId, session.id)],
      );
    }
  }

  void addReasoningChunk(String content) {
    final session = state;
    if (session == null) return;

    _reasoningBuffer.write(content);

    final messages = session.messages;
    if (messages.isNotEmpty && messages.last.role == 'assistant') {
      final updated = List<Message>.from(messages);
      updated[updated.length - 1] = messages.last.copyWith(
        reasoningContent: _reasoningBuffer.toString(),
      );
      state = session.copyWith(messages: updated);
    } else {
      _reasoningBuffer.clear();
      _reasoningBuffer.write(content);
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      state = session.copyWith(
        messages: [...messages, Message.assistant('', null, channelId, userId, session.id, content, false)],
      );
    }
  }

  void completeReasoning() {
    final session = state;
    if (session == null) return;

    _reasoningBuffer.clear();

    final messages = session.messages;
    if (messages.isNotEmpty && messages.last.role == 'assistant') {
      final updated = List<Message>.from(messages);
      updated[updated.length - 1] = messages.last.copyWith(
        isReasoningComplete: true,
      );
      state = session.copyWith(messages: updated);
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
