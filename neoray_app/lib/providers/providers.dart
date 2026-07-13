import 'package:flutter_riverpod/flutter_riverpod.dart';
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
  AppConfigNotifier() : super(const AppConfig());

  void updateConfig(AppConfig config) {
    state = config;
  }

  void updateLLMConfig(LLMConfig config) {
    state = state.copyWith(llm: config);
  }

  void updateChannelConfig(ChannelConfig config) {
    state = state.copyWith(channel: config);
  }

  void updateToolConfig(ToolConfig config) {
    state = state.copyWith(tools: config);
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
    } catch (e) {
      // Handle error
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
        state = state?.copyWith(messages: [...state!.messages, response]);
      } catch (e) {
        // Remove user message from the list on error
        state = state?.copyWith(
          messages: [...state!.messages.where((m) => m != userMessage)],
        );
        rethrow;
      }
    }
  }

  void addStreamingChunk(String content) {
    final session = state;
    if (session == null) return;

    final messages = List<Message>.from(session.messages);
    if (messages.isNotEmpty && messages.last.role == 'assistant') {
      final lastMessage = messages.last;
      messages[messages.length - 1] = lastMessage.copyWith(
        content: lastMessage.content + content,
      );
    } else {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      messages.add(Message.assistant(content, null, channelId, userId, session.id));
    }
    state = session.copyWith(messages: messages);
  }

  void addReasoningChunk(String content) {
    final session = state;
    if (session == null) return;

    final messages = List<Message>.from(session.messages);
    if (messages.isNotEmpty && messages.last.role == 'assistant') {
      final lastMessage = messages.last;
      messages[messages.length - 1] = lastMessage.copyWith(
        reasoningContent: (lastMessage.reasoningContent ?? '') + content,
      );
    } else {
      final channelId = _ref.read(channelIdProvider);
      final userId = _ref.read(userIdProvider);
      messages.add(Message.assistant(
        '',
        null,
        channelId,
        userId,
        session.id,
        content,
        false,
      ));
    }
    state = session.copyWith(messages: messages);
  }

  void completeReasoning() {
    final session = state;
    if (session == null) return;

    final messages = List<Message>.from(session.messages);
    if (messages.isNotEmpty && messages.last.role == 'assistant') {
      final lastMessage = messages.last;
      messages[messages.length - 1] = lastMessage.copyWith(
        isReasoningComplete: true,
      );
      state = session.copyWith(messages: messages);
    }
  }

  void addMessage(Message message) {
    if (state == null) return;
    state = state!.copyWith(messages: [...state!.messages, message]);
  }

  void clearSession() {
    state = null;
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
