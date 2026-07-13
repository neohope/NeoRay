import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';
import '../utils/logger.dart';

enum WebSocketMessageType {
  chatStart,
  chatChunk,
  chatEnd,
  reasoningStart,
  reasoningChunk,
  reasoningEnd,
  toolCallStart,
  toolCallResult,
  sessionCreated,
  sessionJoined,
  sessionList,
  error,
  progress,
  unknown,
}

class WebSocketEvent {
  final WebSocketMessageType type;
  final Map<String, dynamic> data;

  WebSocketEvent({required this.type, required this.data});

  factory WebSocketEvent.fromJson(Map<String, dynamic> json) {
    final type = _parseType(json['type'] as String? ?? '');
    return WebSocketEvent(
        type: type, data: json['payload'] as Map<String, dynamic>? ?? {});
  }

  static WebSocketMessageType _parseType(String type) {
    switch (type) {
      case 'chat_start':
        return WebSocketMessageType.chatStart;
      case 'chat_chunk':
        return WebSocketMessageType.chatChunk;
      case 'chat_end':
        return WebSocketMessageType.chatEnd;
      case 'reasoning_start':
        return WebSocketMessageType.reasoningStart;
      case 'reasoning_chunk':
        return WebSocketMessageType.reasoningChunk;
      case 'reasoning_end':
        return WebSocketMessageType.reasoningEnd;
      case 'tool_call_start':
        return WebSocketMessageType.toolCallStart;
      case 'tool_call_result':
        return WebSocketMessageType.toolCallResult;
      case 'session_created':
        return WebSocketMessageType.sessionCreated;
      case 'session_joined':
        return WebSocketMessageType.sessionJoined;
      case 'session_list':
        return WebSocketMessageType.sessionList;
      case 'error':
        return WebSocketMessageType.error;
      case 'progress':
        return WebSocketMessageType.progress;
      default:
        return WebSocketMessageType.unknown;
    }
  }
}

class WebSocketService {
  WebSocketChannel? _channel;
  final StreamController<WebSocketEvent> _eventController =
      StreamController.broadcast();
  final StreamController<bool> _connectionController =
      StreamController.broadcast();

  bool _isConnected = false;
  bool _disposed = false;
  String? _url;
  String? _sessionId;
  Timer? _reconnectTimer;
  int _reconnectAttempts = 0;
  static const int _maxReconnectAttempts = 10;
  static const Duration _baseReconnectDelay = Duration(seconds: 1);

  Stream<WebSocketEvent> get eventStream => _eventController.stream;
  Stream<bool> get connectionStream => _connectionController.stream;
  bool get isConnected => _isConnected;
  String? get sessionId => _sessionId;

  WebSocketService();

  Future<void> connect(String url) async {
    _url = url;
    _reconnectAttempts = 0;
    await _doConnect(url);
  }

  Future<void> _doConnect(String url) async {
    if (_disposed) return;

    try {
      final wsUrl = url.replaceFirst('http', 'ws');
      _channel = WebSocketChannel.connect(Uri.parse('$wsUrl/ws'));

      // 等待连接就绪后再标记已连接，避免竞态
      await _channel!.ready;

      _channel!.stream.listen(
        _handleMessage,
        onError: _handleError,
        onDone: _handleDone,
      );

      _isConnected = true;
      _reconnectAttempts = 0;
      _connectionController.add(true);

      logger.i('WebSocket connected');
    } catch (e) {
      logger.e('WebSocket connection failed: $e');
      _isConnected = false;
      _connectionController.add(false);
      _scheduleReconnect();
    }
  }

  void _scheduleReconnect() {
    if (_disposed || _url == null) return;
    if (_reconnectAttempts >= _maxReconnectAttempts) {
      logger.w('Max reconnect attempts reached ($_maxReconnectAttempts)');
      return;
    }

    _reconnectTimer?.cancel();
    // Exponential backoff: 1s, 2s, 4s, 8s, ... capped at 30s
    final delay = _baseReconnectDelay * (1 << _reconnectAttempts).clamp(1, 30);
    _reconnectAttempts++;

    logger.i('Reconnecting in ${delay.inSeconds}s (attempt $_reconnectAttempts/$_maxReconnectAttempts)');
    _reconnectTimer = Timer(delay, () {
      if (!_disposed && !_isConnected) {
        _doConnect(_url!);
      }
    });
  }

  void _handleMessage(dynamic message) {
    try {
      final data = jsonDecode(message as String) as Map<String, dynamic>;
      final event = WebSocketEvent.fromJson(data);
      _eventController.add(event);
    } catch (e) {
      logger.e('Failed to parse WebSocket message: $e');
    }
  }

  void _handleError(dynamic error) {
    logger.e('WebSocket error: $error');
    _isConnected = false;
    _connectionController.add(false);
    _scheduleReconnect();
  }

  void _handleDone() {
    logger.w('WebSocket connection closed');
    _isConnected = false;
    _connectionController.add(false);
    if (!_disposed) {
      _scheduleReconnect();
    }
  }

  void sendChat({
    required String message,
    String? channelId,
    String? userId,
    String? sessionId,
    bool stream = true,
  }) {
    if (!_isConnected) {
      logger.w('WebSocket not connected');
      return;
    }

    final payload = {
      'type': stream ? 'chat_stream' : 'chat',
      'payload': {
        if (channelId != null) 'channel_id': channelId,
        if (userId != null) 'user_id': userId,
        if (sessionId != null) 'session_id': sessionId,
        'message': message,
      },
    };

    _channel!.sink.add(jsonEncode(payload));
  }

  void createSession({String? channelId, String? userId, String? title}) {
    if (!_isConnected) return;

    final payload = {
      'type': 'create_session',
      'payload': {
        if (channelId != null) 'channel_id': channelId,
        if (userId != null) 'user_id': userId,
        'name': title ?? '新聊天',
      },
    };

    _channel!.sink.add(jsonEncode(payload));
  }

  void joinSession(String sessionId, {String? channelId, String? userId}) {
    if (!_isConnected) return;

    _sessionId = sessionId;
    final payload = {
      'type': 'join_session',
      'payload': {
        if (channelId != null) 'channel_id': channelId,
        if (userId != null) 'user_id': userId,
        'session_id': sessionId,
      },
    };

    _channel!.sink.add(jsonEncode(payload));
  }

  void listSessions({String? channelId, String? userId}) {
    if (!_isConnected) return;

    final payload = {
      'type': 'list_sessions',
      'payload': {
        if (channelId != null) 'channel_id': channelId,
        if (userId != null) 'user_id': userId,
      },
    };
    _channel!.sink.add(jsonEncode(payload));
  }

  Future<void> disconnect() async {
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    await _channel?.sink.close();
    _isConnected = false;
    _connectionController.add(false);
  }

  void dispose() {
    _disposed = true;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _channel?.sink.close();
    _isConnected = false;
    _eventController.close();
    _connectionController.close();
  }
}
