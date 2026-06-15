import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/session.dart';
import '../models/message.dart';
import '../models/app_config.dart';
import '../services/api_service.dart';
import '../services/websocket_service.dart';
import '../utils/logger.dart';

// API Service Provider
final apiServiceProvider = Provider<ApiService>((ref) {
  return ApiService(baseUrl: 'http://localhost:8080');
});

// Global Error Provider
final globalErrorProvider = StateProvider<String?>((ref) => null);

// WebSocket Service Provider
final webSocketServiceProvider = Provider<WebSocketService>((ref) {
  return WebSocketService();
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
  return SessionListNotifier(apiService);
});

class SessionListNotifier extends StateNotifier<AsyncValue<List<Session>>> {
  final ApiService _apiService;

  SessionListNotifier(this._apiService) : super(const AsyncValue.loading()) {
    loadSessions();
  }

  Future<void> loadSessions() async {
    state = const AsyncValue.loading();
    try {
      final sessions = await _apiService.getSessions();
      state = AsyncValue.data(sessions);
    } catch (e, stackTrace) {
      state = AsyncValue.error(e, stackTrace);
    }
  }

  Future<void> createSession({String? title}) async {
    try {
      await _apiService.createSession(title: title);
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
  return CurrentSessionNotifier(apiService, webSocket);
});

class CurrentSessionNotifier extends StateNotifier<Session?> {
  final ApiService _apiService;
  final WebSocketService _webSocketService;

  CurrentSessionNotifier(this._apiService, this._webSocketService) : super(null);

  Future<void> selectSession(String sessionId) async {
    try {
      final session = await _apiService.getSession(sessionId);
      state = session;
      _webSocketService.joinSession(sessionId);
    } catch (e, stackTrace) {
      logger.e('选择会话失败', error: e, stackTrace: stackTrace);
      // 保持当前状态不变，错误由调用方处理
      rethrow;
    }
  }

  Future<void> newSession({String? title}) async {
    try {
      final session = Session.create(title: title);
      state = session;
    } catch (e) {
      // Handle error
    }
  }

  Future<void> sendMessage(String message) async {
    final session = state;
    if (session == null) return;

    final userMessage = Message.user(message);
    state = session.copyWith(messages: [...session.messages, userMessage]);

    if (_webSocketService.isConnected) {
      _webSocketService.sendChat(
        message: message,
        sessionId: session.id,
        stream: true,
      );
    } else {
      try {
        final response = await _apiService.sendMessage(
          sessionId: session.id,
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
      messages.add(Message.assistant(content));
    }
    state = session.copyWith(messages: messages);
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

// UI State Providers
final activePageProvider = StateProvider<AppPage>((ref) => AppPage.chat);
final isSettingsVisibleProvider = StateProvider<bool>((ref) => false);

enum AppPage {
  chat,
  config,
}
