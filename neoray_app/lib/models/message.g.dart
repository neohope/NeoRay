// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_$ToolCallImpl _$$ToolCallImplFromJson(Map<String, dynamic> json) =>
    _$ToolCallImpl(
      id: json['id'] as String,
      name: json['name'] as String,
      arguments: json['arguments'] as String,
    );

Map<String, dynamic> _$$ToolCallImplToJson(_$ToolCallImpl instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'arguments': instance.arguments,
    };

_$MessageImpl _$$MessageImplFromJson(Map<String, dynamic> json) =>
    _$MessageImpl(
      role: json['role'] as String,
      content: json['content'] as String,
      channelId: json['channelId'] as String?,
      userId: json['userId'] as String?,
      sessionId: json['sessionId'] as String?,
      toolCalls: (json['toolCalls'] as List<dynamic>?)
              ?.map((e) => ToolCall.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
      timestamp: json['timestamp'] == null
          ? null
          : DateTime.parse(json['timestamp'] as String),
    );

Map<String, dynamic> _$$MessageImplToJson(_$MessageImpl instance) =>
    <String, dynamic>{
      'role': instance.role,
      'content': instance.content,
      'channelId': instance.channelId,
      'userId': instance.userId,
      'sessionId': instance.sessionId,
      'toolCalls': instance.toolCalls,
      'timestamp': instance.timestamp?.toIso8601String(),
    };
