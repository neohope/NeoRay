import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:json_annotation/json_annotation.dart';

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
    @Default([]) List<ToolCall> toolCalls,
    DateTime? timestamp,
  }) = _Message;

  factory Message.user(String content) => Message(
        role: MessageRole.user.name,
        content: content,
        timestamp: DateTime.now(),
      );

  factory Message.assistant(String content, [List<ToolCall>? toolCalls]) =>
      Message(
        role: MessageRole.assistant.name,
        content: content,
        toolCalls: toolCalls ?? [],
        timestamp: DateTime.now(),
      );

  factory Message.tool(String content) => Message(
        role: MessageRole.tool.name,
        content: content,
        timestamp: DateTime.now(),
      );

  factory Message.fromJson(Map<String, dynamic> json) =>
      _$MessageFromJson(json);
}
