import 'package:freezed_annotation/freezed_annotation.dart';

part 'message.freezed.dart';
part 'message.g.dart';

enum MessageRole {
  @JsonValue('user')
  user,
  @JsonValue('assistant')
  assistant,
  @JsonValue('tool')
  tool,
}

@freezed
class ToolCall with _$ToolCall {
  const factory ToolCall({
    required String id,
    required String name,
    required String arguments,
  }) = _ToolCall;

  factory ToolCall.fromJson(Map<String, dynamic> json) =>
      _$ToolCallFromJson(json);
}

@freezed
class Message with _$Message {
  const factory Message({
    required String role,
    required String content,
    String? channelId,
    String? userId,
    String? sessionId,
    @Default([]) List<ToolCall> toolCalls,
    DateTime? timestamp,
  }) = _Message;

  factory Message.user(String content, {String? channelId, String? userId, String? sessionId}) => Message(
        role: MessageRole.user.name,
        content: content,
        channelId: channelId,
        userId: userId,
        sessionId: sessionId,
        timestamp: DateTime.now(),
      );

  factory Message.assistant(String content, [List<ToolCall>? toolCalls, String? channelId, String? userId, String? sessionId]) =>
      Message(
        role: MessageRole.assistant.name,
        content: content,
        channelId: channelId,
        userId: userId,
        sessionId: sessionId,
        toolCalls: toolCalls ?? [],
        timestamp: DateTime.now(),
      );

  factory Message.tool(String content, {String? channelId, String? userId, String? sessionId}) => Message(
        role: MessageRole.tool.name,
        content: content,
        channelId: channelId,
        userId: userId,
        sessionId: sessionId,
        timestamp: DateTime.now(),
      );

  factory Message.fromJson(Map<String, dynamic> json) =>
      _$MessageFromJson(json);
}
