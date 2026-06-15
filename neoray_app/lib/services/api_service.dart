import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/session.dart';
import '../models/message.dart';
import '../utils/logger.dart';

class ApiService {
  final String baseUrl;
  final http.Client _httpClient;
  final Duration timeout;

  ApiService({
    required this.baseUrl,
    http.Client? httpClient,
    this.timeout = const Duration(seconds: 120),
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

  Future<List<Session>> getSessions() async {
    return _executeRequest(() async {
      final response = await _httpClient
          .get(_buildUri('/api/sessions'))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      final sessionsJson = data['sessions'] as List? ?? [];
      return sessionsJson
          .map((json) => Session.fromJson(json as Map<String, dynamic>))
          .toList();
    });
  }

  Future<Session> createSession({String? title}) async {
    return _executeRequest(() async {
      final response = await _httpClient
          .post(
            _buildUri('/api/sessions'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({'name': title ?? '新聊天'}),
          )
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return Session.fromJson(data);
    });
  }

  Future<Session> getSession(String sessionId) async {
    return _executeRequest(() async {
      final response = await _httpClient
          .get(_buildUri('/api/sessions/$sessionId'))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return Session.fromJson(data);
    });
  }

  Future<Message> sendMessage({
    required String sessionId,
    required String message,
    bool stream = false,
  }) async {
    return _executeRequest(() async {
      final response = await _httpClient
          .post(
            _buildUri('/api/sessions/$sessionId'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({'message': message, 'stream': stream}),
          )
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
      final data = await _handleResponse(response);
      return Message.assistant(data['content'] as String? ?? '');
    });
  }

  Future<void> deleteSession(String sessionId) async {
    return _executeRequest(() async {
      await _httpClient
          .delete(_buildUri('/api/sessions/$sessionId'))
          .timeout(timeout, onTimeout: () => throw TimeoutException('请求超时'));
    });
  }

  WebSocketChannel connectWebSocket() {
    final wsUrl = baseUrl.replaceFirst('http', 'ws');
    return WebSocketChannel.connect(Uri.parse('$wsUrl/ws'));
  }
}

class ApiException implements Exception {
  final int statusCode;
  final String message;

  ApiException({required this.statusCode, required this.message});

  @override
  String toString() => 'ApiException($statusCode): $message';
}

class TimeoutException implements Exception {
  final String message;
  TimeoutException(this.message);

  @override
  String toString() => message;
}

class NetworkException implements Exception {
  final String message;
  NetworkException(this.message);

  @override
  String toString() => message;
}
