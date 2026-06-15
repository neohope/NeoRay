import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/message.dart';
import '../utils/logger.dart';

enum WebSocketMessageType {
  chatStart,
  chatChunk,
  chatEnd,
  toolCallStart,
  toolCallResult,
  sessionCreated,
  sessionJoined,
  sessionList,
  error,
  unknown,
}

class WebSocketEvent {
  final WebSocketMessageType type;
  final Map<String, dynamic> data;

  WebSocketEvent({required this.type, required this.data});

  factory WebSocketEvent.fromJson(Map<String, dynamic> json) {
    final type = _parseType(json['type'] as String? ?? '');
    return WebSocketEvent(type: type, data: json['payload'] as Map<String, dynamic>? ?? {});
  }

  static WebSocketMessageType _parseType(String type) {
    switch (type) {
      case 'chat_start':
        return WebSocketMessageType.chatStart;
      case 'chat_chunk':
        return WebSocketMessageType.chatChunk;
      case 'chat_end':
        return WebSocketMessageType.chatEnd;
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
  String? _sessionId;

  Stream<WebSocketEvent> get eventStream => _eventController.stream;
  Stream<bool> get connectionStream => _connectionController.stream;
  bool get isConnected => _isConnected;
  String? get sessionId => _sessionId;

  WebSocketService();

  Future<void> connect(String url) async {
    try {
      final wsUrl = url.replaceFirst('http', 'ws');
      _channel = WebSocketChannel.connect(Uri.parse('$wsUrl/ws'));

      _isConnected = true;
      _connectionController.add(true);

      _channel!.stream.listen(
        _handleMessage,
        onError: _handleError,
        onDone: _handleDone,
      );

      logger.i('WebSocket connected');
    } catch (e) {
      logger.e('WebSocket connection failed: $e');
      _isConnected = false;
      _connectionController.add(false);
    }
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
  }

  void _handleDone() {
    logger.w('WebSocket connection closed');
    _isConnected = false;
    _connectionController.add(false);
  }

  void sendChat({
    required String message,
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
        if (sessionId != null) 'session_id': sessionId,
        'message': message,
      },
    };

    _channel!.sink.add(jsonEncode(payload));
  }

  void createSession({String? title}) {
    if (!_isConnected) return;

    final payload = {
      'type': 'create_session',
      'payload': {'name': title ?? '新聊天'},
    };

    _channel!.sink.add(jsonEncode(payload));
  }

  void joinSession(String sessionId) {
    if (!_isConnected) return;

    _sessionId = sessionId;
    final payload = {
      'type': 'join_session',
      'payload': {'session_id': sessionId},
    };

    _channel!.sink.add(jsonEncode(payload));
  }

  void listSessions() {
    if (!_isConnected) return;

    const payload = {'type': 'list_sessions'};
    _channel!.sink.add(jsonEncode(payload));
  }

  Future<void> disconnect() async {
    await _channel?.sink.close();
    _isConnected = false;
    _connectionController.add(false);
  }

  void dispose() {
    disconnect();
    _eventController.close();
    _connectionController.close();
  }
}
