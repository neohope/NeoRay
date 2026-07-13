// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_config.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_$LLMConfigImpl _$$LLMConfigImplFromJson(Map<String, dynamic> json) =>
    _$LLMConfigImpl(
      provider: json['provider'] as String? ?? AppDefaults.defaultLLMProvider,
      apiKey: json['apiKey'] as String? ?? '',
      apiUrl: json['apiUrl'] as String? ?? AppDefaults.defaultApiUrl,
      model: json['model'] as String? ?? AppDefaults.defaultLLMModel,
      maxTokens:
          (json['maxTokens'] as num?)?.toInt() ?? AppDefaults.defaultMaxTokens,
      temperature: (json['temperature'] as num?)?.toDouble() ??
          AppDefaults.defaultTemperature,
      timeout:
          (json['timeout'] as num?)?.toInt() ?? AppDefaults.defaultTimeoutSec,
      reasoningEffort: json['reasoningEffort'] as String? ??
          AppDefaults.defaultReasoningEffort,
      promptCacheEnabled: json['promptCacheEnabled'] as bool? ??
          AppDefaults.defaultPromptCacheEnabled,
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
      enabled: json['enabled'] as bool? ?? AppDefaults.defaultChannelEnabled,
      provider:
          json['provider'] as String? ?? AppDefaults.defaultChannelProvider,
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
      shellEnabled:
          json['shellEnabled'] as bool? ?? AppDefaults.defaultShellEnabled,
      cronEnabled:
          json['cronEnabled'] as bool? ?? AppDefaults.defaultCronEnabled,
      webEnabled: json['webEnabled'] as bool? ?? AppDefaults.defaultWebEnabled,
    );

Map<String, dynamic> _$$ToolConfigImplToJson(_$ToolConfigImpl instance) =>
    <String, dynamic>{
      'shellEnabled': instance.shellEnabled,
      'cronEnabled': instance.cronEnabled,
      'webEnabled': instance.webEnabled,
    };

_$AppConfigImpl _$$AppConfigImplFromJson(Map<String, dynamic> json) =>
    _$AppConfigImpl(
      serverUrl: json['serverUrl'] as String? ?? AppDefaults.defaultServerUrl,
      themeMode: json['themeMode'] as String? ?? AppDefaults.defaultThemeMode,
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
      'themeMode': instance.themeMode,
      'llm': instance.llm,
      'channel': instance.channel,
      'tools': instance.tools,
    };
