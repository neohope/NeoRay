import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/session.dart';
import '../models/message.dart';

class ApiService {
  final String baseUrl;
  final http.Client _httpClient;

  ApiService({required this.baseUrl, http.Client? httpClient})
      : _httpClient = httpClient ?? http.Client();

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
    throw ApiException(
      statusCode: response.statusCode,
      message: response.body,
    );
  }

  Future<Map<String, dynamic>> getHealth() async {
    final response = await _httpClient.get(_buildUri('/api/health'));
    return _handleResponse(response);
  }

  Future<List<Session>> getSessions() async {
    final response = await _httpClient.get(_buildUri('/api/sessions'));
    final data = await _handleResponse(response);
    final sessionsJson = data['sessions'] as List? ?? [];
    return sessionsJson
        .map((json) => Session.fromJson(json as Map<String, dynamic>))
        .toList();
  }

  Future<Session> createSession({String? title}) async {
    final response = await _httpClient.post(
      _buildUri('/api/sessions'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'name': title ?? '新聊天'}),
    );
    final data = await _handleResponse(response);
    return Session.fromJson(data);
  }

  Future<Session> getSession(String sessionId) async {
    final response = await _httpClient.get(_buildUri('/api/sessions/$sessionId'));
    final data = await _handleResponse(response);
    return Session.fromJson(data);
  }

  Future<Message> sendMessage({
    required String sessionId,
    required String message,
    bool stream = false,
  }) async {
    final response = await _httpClient.post(
      _buildUri('/api/sessions/$sessionId'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'message': message, 'stream': stream}),
    );
    final data = await _handleResponse(response);
    return Message.assistant(data['content'] as String? ?? '');
  }

  Future<void> deleteSession(String sessionId) async {
    await _httpClient.delete(_buildUri('/api/sessions/$sessionId'));
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
