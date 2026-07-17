// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_config.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_$AppConfigImpl _$$AppConfigImplFromJson(Map<String, dynamic> json) =>
    _$AppConfigImpl(
      serverUrl: json['serverUrl'] as String? ?? AppDefaults.defaultServerUrl,
      themeMode: json['themeMode'] as String? ?? AppDefaults.defaultThemeMode,
      userId: json['userId'] as String? ?? '',
      channelId: json['channelId'] as String? ?? '',
      apiKey: json['apiKey'] as String? ?? '',
    );

Map<String, dynamic> _$$AppConfigImplToJson(_$AppConfigImpl instance) =>
    <String, dynamic>{
      'serverUrl': instance.serverUrl,
      'themeMode': instance.themeMode,
      'userId': instance.userId,
      'channelId': instance.channelId,
      'apiKey': instance.apiKey,
    };

_$ServerConfigImpl _$$ServerConfigImplFromJson(Map<String, dynamic> json) =>
    _$ServerConfigImpl(
      llm: json['llm'] == null
          ? const ServerLLMConfig()
          : ServerLLMConfig.fromJson(json['llm'] as Map<String, dynamic>),
      channels: json['channels'] == null
          ? const ServerChannelConfig()
          : ServerChannelConfig.fromJson(
              json['channels'] as Map<String, dynamic>),
      tools: json['tools'] == null
          ? const ServerToolConfig()
          : ServerToolConfig.fromJson(json['tools'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$$ServerConfigImplToJson(_$ServerConfigImpl instance) =>
    <String, dynamic>{
      'llm': instance.llm,
      'channels': instance.channels,
      'tools': instance.tools,
    };

_$ServerLLMConfigImpl _$$ServerLLMConfigImplFromJson(
        Map<String, dynamic> json) =>
    _$ServerLLMConfigImpl(
      defaultProvider:
          json['defaultProvider'] as String? ?? AppDefaults.defaultLLMProvider,
      providers: (json['providers'] as Map<String, dynamic>?)?.map(
            (k, e) =>
                MapEntry(k, ProviderConfig.fromJson(e as Map<String, dynamic>)),
          ) ??
          const {},
      fallbackModels: (json['fallbackModels'] as List<dynamic>?)
              ?.map((e) =>
                  FallbackModelConfig.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
    );

Map<String, dynamic> _$$ServerLLMConfigImplToJson(
        _$ServerLLMConfigImpl instance) =>
    <String, dynamic>{
      'defaultProvider': instance.defaultProvider,
      'providers': instance.providers,
      'fallbackModels': instance.fallbackModels,
    };

_$ProviderConfigImpl _$$ProviderConfigImplFromJson(Map<String, dynamic> json) =>
    _$ProviderConfigImpl(
      apiKey: json['apiKey'] as String? ?? '',
      apiUrl: json['apiUrl'] as String? ?? '',
      model: json['model'] as String? ?? AppDefaults.defaultLLMModel,
      maxTokens:
          (json['maxTokens'] as num?)?.toInt() ?? AppDefaults.defaultMaxTokens,
      temperature: (json['temperature'] as num?)?.toDouble() ??
          AppDefaults.defaultTemperature,
      timeout: (json['timeout'] as num?)?.toDouble() ??
          AppDefaults.defaultTimeoutSec,
      reasoningEffort: json['reasoningEffort'] as String? ??
          AppDefaults.defaultReasoningEffort,
      promptCacheEnabled: json['promptCacheEnabled'] as bool? ??
          AppDefaults.defaultPromptCacheEnabled,
    );

Map<String, dynamic> _$$ProviderConfigImplToJson(
        _$ProviderConfigImpl instance) =>
    <String, dynamic>{
      'apiKey': instance.apiKey,
      'apiUrl': instance.apiUrl,
      'model': instance.model,
      'maxTokens': instance.maxTokens,
      'temperature': instance.temperature,
      'timeout': instance.timeout,
      'reasoningEffort': instance.reasoningEffort,
      'promptCacheEnabled': instance.promptCacheEnabled,
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

_$ServerChannelConfigImpl _$$ServerChannelConfigImplFromJson(
        Map<String, dynamic> json) =>
    _$ServerChannelConfigImpl(
      feishu: json['feishu'] == null
          ? const FeishuConfig()
          : FeishuConfig.fromJson(json['feishu'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$$ServerChannelConfigImplToJson(
        _$ServerChannelConfigImpl instance) =>
    <String, dynamic>{
      'feishu': instance.feishu,
    };

_$FeishuConfigImpl _$$FeishuConfigImplFromJson(Map<String, dynamic> json) =>
    _$FeishuConfigImpl(
      enabled: json['enabled'] as bool? ?? false,
      appId: json['appId'] as String? ?? '',
      appSecret: json['appSecret'] as String? ?? '',
    );

Map<String, dynamic> _$$FeishuConfigImplToJson(_$FeishuConfigImpl instance) =>
    <String, dynamic>{
      'enabled': instance.enabled,
      'appId': instance.appId,
      'appSecret': instance.appSecret,
    };

_$ServerToolConfigImpl _$$ServerToolConfigImplFromJson(
        Map<String, dynamic> json) =>
    _$ServerToolConfigImpl(
      shell: json['shell'] == null
          ? const ToolEnabledConfig()
          : ToolEnabledConfig.fromJson(json['shell'] as Map<String, dynamic>),
      web: json['web'] == null
          ? const ToolEnabledConfig()
          : ToolEnabledConfig.fromJson(json['web'] as Map<String, dynamic>),
      cron: json['cron'] == null
          ? const ToolEnabledConfig()
          : ToolEnabledConfig.fromJson(json['cron'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$$ServerToolConfigImplToJson(
        _$ServerToolConfigImpl instance) =>
    <String, dynamic>{
      'shell': instance.shell,
      'web': instance.web,
      'cron': instance.cron,
    };

_$ToolEnabledConfigImpl _$$ToolEnabledConfigImplFromJson(
        Map<String, dynamic> json) =>
    _$ToolEnabledConfigImpl(
      enabled: json['enabled'] as bool? ?? true,
    );

Map<String, dynamic> _$$ToolEnabledConfigImplToJson(
        _$ToolEnabledConfigImpl instance) =>
    <String, dynamic>{
      'enabled': instance.enabled,
    };
