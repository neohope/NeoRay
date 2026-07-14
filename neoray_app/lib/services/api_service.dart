import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/session.dart';
import '../models/message.dart';
import '../models/app_config.dart';
import '../constants/constants.dart';
import '../utils/logger.dart';

class ApiService {
  final String baseUrl;
  final http.Client _httpClient;
  final Duration timeout;
  final String channelId;
  final String userId;

  ApiService({
    required this.baseUrl,
    required this.channelId,
    required this.userId,
    http.Client? httpClient,
    this.timeout = const Duration(seconds: AppTimings.apiTimeoutSec),
  }) : _httpClient = httpClient ?? http.Client();

  Uri _buildUri(String path, [Map<String, String>? queryParams]) {
    return Uri.parse('$baseUrl$path').replace(queryParameters: queryParams);
  }

  Future<Map<String, dynamic>> _handleResponse(http.Response response) async {
    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (response.body.isEmpty) return {};
      try {
        return jsonDecode(response.body) as Map<String, dynamic>;
      } catch (e) {
        return {};
      }
    }
    String message = response.body;
    try {
      final data = jsonDecode(response.body);
      message = data['message'] ?? data['error'] ?? response.body;
    } catch (_) {}
    throw ApiException(
      statusCode: response.statusCode,
      message: message,
    );
  }

  Future<T> _executeRequest<T>(Future<T> Function() request) async {
    try {
      return await request();
    } on TimeoutException catch (e) {
      logger.w('请求超时', error: e);
      rethrow;
    } on SocketException catch (e) {
      logger.e('网络连接失败', error: e);
      throw NetworkException('无法连接到服务器，请检查网络设置');
    } on HttpException catch (e) {
      logger.e('HTTP请求异常', error: e);
      throw NetworkException('请求失败: ${e.message}');
    } on FormatException catch (e) {
      logger.e('响应格式错误', error: e);
      throw ApiException(statusCode: 0, message: '服务器响应格式错误');
    } catch (e) {
      logger.e('请求失败', error: e);
      if (e is ApiException || e is TimeoutException || e is NetworkException) {
        rethrow;
      }
      throw NetworkException('请求失败: $e');
    }
  }

  Future<Map<String, dynamic>> getHealth() async {
    return _executeRequest(() async {
      final response = await _httpClient
          .get(_buildUri('/api/health'))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      return _handleResponse(response);
    });
  }

  Future<List<Session>> getSessions({String? channelId, String? userId}) async {
    return _executeRequest(() async {
      final queryParams = <String, String>{};
      if (channelId != null) queryParams['channel_id'] = channelId;
      if (userId != null) queryParams['user_id'] = userId;
      final response = await _httpClient
          .get(_buildUri('/api/sessions', queryParams))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      final sessionsJson = data['sessions'] as List? ?? [];
      return sessionsJson
          .map((json) => Session.fromJson(json as Map<String, dynamic>))
          .toList();
    });
  }

  Future<Session> createSession({String? channelId, String? userId, String? title}) async {
    return _executeRequest(() async {
      final response = await _httpClient
          .post(
            _buildUri('/api/sessions'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({
              'channel_id': channelId ?? this.channelId,
              'user_id': userId ?? this.userId,
              'name': title ?? '新聊天',
            }),
          )
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return Session.fromJson(data);
    });
  }

  Future<Session> getSession(String sessionId, {String? channelId, String? userId}) async {
    return _executeRequest(() async {
      final queryParams = <String, String>{};
      if (channelId != null) queryParams['channel_id'] = channelId;
      if (userId != null) queryParams['user_id'] = userId;
      final response = await _httpClient
          .get(_buildUri('/api/sessions/$sessionId', queryParams))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return Session.fromJson(data);
    });
  }

  Future<Message> sendMessage({
    required String sessionId,
    required String message,
    String? channelId,
    String? userId,
    bool stream = false,
  }) async {
    return _executeRequest(() async {
      final response = await _httpClient
          .post(
            _buildUri('/api/sessions/$sessionId'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({
              'channel_id': channelId ?? this.channelId,
              'user_id': userId ?? this.userId,
              'message': message,
              'stream': stream,
            }),
          )
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return Message.assistant(
        data['content'] as String? ?? '',
        null,
        channelId ?? this.channelId,
        userId ?? this.userId,
        sessionId,
      );
    });
  }

  Future<void> deleteSession(String sessionId) async {
    return _executeRequest(() async {
      await _httpClient
          .delete(_buildUri('/api/sessions/$sessionId'))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
    });
  }

  Future<ServerConfig> getServerConfig() async {
    return _executeRequest(() async {
      final response = await _httpClient
          .get(_buildUri('/api/config'))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return ServerConfig.fromJson(data);
    });
  }

  Future<ServerConfig> updateServerConfig(Map<String, dynamic> updates) async {
    return _executeRequest(() async {
      final response = await _httpClient
          .put(
            _buildUri('/api/config'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode(updates),
          )
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return ServerConfig.fromJson(data);
    });
  }

  WebSocketChannel connectWebSocket() {
    final uri = Uri.parse(baseUrl);
    final wsUri = uri.replace(scheme: uri.scheme == 'https' ? 'wss' : 'ws', path: '${uri.path}/ws');
    return WebSocketChannel.connect(wsUri);
  }
}

class ApiException implements Exception {
  final int statusCode;
  final String message;

  ApiException({required this.statusCode, required this.message});

  @override
  String toString() => 'ApiException($statusCode): $message';
}

class NetworkException implements Exception {
  final String message;
  NetworkException(this.message);

  @override
  String toString() => message;
}
