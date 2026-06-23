// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_config.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_$LLMConfigImpl _$$LLMConfigImplFromJson(Map<String, dynamic> json) =>
    _$LLMConfigImpl(
      provider: json['provider'] as String? ?? 'openai',
      apiKey: json['apiKey'] as String? ?? '',
      apiUrl: json['apiUrl'] as String? ?? 'https://api.openai.com/v1',
      model: json['model'] as String? ?? 'gpt-4',
      maxTokens: (json['maxTokens'] as num?)?.toInt() ?? 4096,
      temperature: (json['temperature'] as num?)?.toDouble() ?? 0.7,
      timeout: (json['timeout'] as num?)?.toInt() ?? 120,
      reasoningEffort: json['reasoningEffort'] as String? ?? 'none',
      promptCacheEnabled: json['promptCacheEnabled'] as bool? ?? false,
      fallbackModels: (json['fallbackModels'] as List<dynamic>?)
              ?.map((e) =>
                  FallbackModelConfig.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
    );

Map<String, dynamic> _$$LLMConfigImplToJson(_$LLMConfigImpl instance) =>
    <String, dynamic>{
      'provider': instance.provider,
      'apiKey': instance.apiKey,
      'apiUrl': instance.apiUrl,
      'model': instance.model,
      'maxTokens': instance.maxTokens,
      'temperature': instance.temperature,
      'timeout': instance.timeout,
      'reasoningEffort': instance.reasoningEffort,
      'promptCacheEnabled': instance.promptCacheEnabled,
      'fallbackModels': instance.fallbackModels,
    };

_$FallbackModelConfigImpl _$$FallbackModelConfigImplFromJson(
        Map<String, dynamic> json) =>
    _$FallbackModelConfigImpl(
      model: json['model'] as String?,
      provider: json['provider'] as String?,
      maxTokens: (json['maxTokens'] as num?)?.toInt(),
      temperature: (json['temperature'] as num?)?.toDouble(),
      reasoningEffort: json['reasoningEffort'] as String?,
    );

Map<String, dynamic> _$$FallbackModelConfigImplToJson(
        _$FallbackModelConfigImpl instance) =>
    <String, dynamic>{
      'model': instance.model,
      'provider': instance.provider,
      'maxTokens': instance.maxTokens,
      'temperature': instance.temperature,
      'reasoningEffort': instance.reasoningEffort,
    };

_$ChannelConfigImpl _$$ChannelConfigImplFromJson(Map<String, dynamic> json) =>
    _$ChannelConfigImpl(
      enabled: json['enabled'] as bool? ?? false,
      provider: json['provider'] as String? ?? 'feishu',
      appId: json['appId'] as String? ?? '',
      appSecret: json['appSecret'] as String? ?? '',
    );

Map<String, dynamic> _$$ChannelConfigImplToJson(_$ChannelConfigImpl instance) =>
    <String, dynamic>{
      'enabled': instance.enabled,
      'provider': instance.provider,
      'appId': instance.appId,
      'appSecret': instance.appSecret,
    };

_$ToolConfigImpl _$$ToolConfigImplFromJson(Map<String, dynamic> json) =>
    _$ToolConfigImpl(
      shellEnabled: json['shellEnabled'] as bool? ?? true,
      cronEnabled: json['cronEnabled'] as bool? ?? true,
      webEnabled: json['webEnabled'] as bool? ?? false,
    );

Map<String, dynamic> _$$ToolConfigImplToJson(_$ToolConfigImpl instance) =>
    <String, dynamic>{
      'shellEnabled': instance.shellEnabled,
      'cronEnabled': instance.cronEnabled,
      'webEnabled': instance.webEnabled,
    };

_$AppConfigImpl _$$AppConfigImplFromJson(Map<String, dynamic> json) =>
    _$AppConfigImpl(
      serverUrl: json['serverUrl'] as String? ?? 'http://localhost:8080',
      llm: json['llm'] == null
          ? const LLMConfig()
          : LLMConfig.fromJson(json['llm'] as Map<String, dynamic>),
      channel: json['channel'] == null
          ? const ChannelConfig()
          : ChannelConfig.fromJson(json['channel'] as Map<String, dynamic>),
      tools: json['tools'] == null
          ? const ToolConfig()
          : ToolConfig.fromJson(json['tools'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$$AppConfigImplToJson(_$AppConfigImpl instance) =>
    <String, dynamic>{
      'serverUrl': instance.serverUrl,
      'llm': instance.llm,
      'channel': instance.channel,
      'tools': instance.tools,
    };
