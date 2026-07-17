import 'package:freezed_annotation/freezed_annotation.dart';
import '../constants/constants.dart';

part 'app_config.freezed.dart';
part 'app_config.g.dart';

// ─── 客户端本地配置（服务端不需要） ───

@freezed
class AppConfig with _$AppConfig {
  const factory AppConfig({
    @Default(AppDefaults.defaultServerUrl) String serverUrl,
    @Default(AppDefaults.defaultThemeMode) String themeMode,
    @Default('') String userId,
    @Default('') String channelId,
    @Default('') String apiKey,
  }) = _AppConfig;

  factory AppConfig.fromJson(Map<String, dynamic> json) =>
      _$AppConfigFromJson(json);
}

// ─── 服务端配置（客户端仅展示/编辑，通过 API 读写） ───

@freezed
class ServerConfig with _$ServerConfig {
  const factory ServerConfig({
    @Default(ServerLLMConfig()) ServerLLMConfig llm,
    @Default(ServerChannelConfig()) ServerChannelConfig channels,
    @Default(ServerToolConfig()) ServerToolConfig tools,
  }) = _ServerConfig;

  factory ServerConfig.fromJson(Map<String, dynamic> json) =>
      _$ServerConfigFromJson(json);
}

@freezed
class ServerLLMConfig with _$ServerLLMConfig {
  const factory ServerLLMConfig({
    @Default(AppDefaults.defaultLLMProvider) String defaultProvider,
    @Default({}) Map<String, ProviderConfig> providers,
    @Default([]) List<FallbackModelConfig> fallbackModels,
  }) = _ServerLLMConfig;

  factory ServerLLMConfig.fromJson(Map<String, dynamic> json) =>
      _$ServerLLMConfigFromJson(json);
}

@freezed
class ProviderConfig with _$ProviderConfig {
  const factory ProviderConfig({
    @Default('') String apiKey,
    @Default('') String apiUrl,
    @Default(AppDefaults.defaultLLMModel) String model,
    @Default(AppDefaults.defaultMaxTokens) int maxTokens,
    @Default(AppDefaults.defaultTemperature) double temperature,
    @Default(AppDefaults.defaultTimeoutSec) double timeout,
    @Default(AppDefaults.defaultReasoningEffort) String reasoningEffort,
    @Default(AppDefaults.defaultPromptCacheEnabled) bool promptCacheEnabled,
  }) = _ProviderConfig;

  factory ProviderConfig.fromJson(Map<String, dynamic> json) =>
      _$ProviderConfigFromJson(json);
}

@freezed
class FallbackModelConfig with _$FallbackModelConfig {
  const factory FallbackModelConfig({
    String? model,
    String? provider,
    int? maxTokens,
    double? temperature,
    String? reasoningEffort,
  }) = _FallbackModelConfig;

  factory FallbackModelConfig.fromJson(Map<String, dynamic> json) =>
      _$FallbackModelConfigFromJson(json);
}

@freezed
class ServerChannelConfig with _$ServerChannelConfig {
  const factory ServerChannelConfig({
    @Default(FeishuConfig()) FeishuConfig feishu,
  }) = _ServerChannelConfig;

  factory ServerChannelConfig.fromJson(Map<String, dynamic> json) =>
      _$ServerChannelConfigFromJson(json);
}

@freezed
class FeishuConfig with _$FeishuConfig {
  const factory FeishuConfig({
    @Default(false) bool enabled,
    @Default('') String appId,
    @Default('') String appSecret,
  }) = _FeishuConfig;

  factory FeishuConfig.fromJson(Map<String, dynamic> json) =>
      _$FeishuConfigFromJson(json);
}

@freezed
class ServerToolConfig with _$ServerToolConfig {
  const factory ServerToolConfig({
    @Default(ToolEnabledConfig()) ToolEnabledConfig shell,
    @Default(ToolEnabledConfig()) ToolEnabledConfig web,
    @Default(ToolEnabledConfig()) ToolEnabledConfig cron,
  }) = _ServerToolConfig;

  factory ServerToolConfig.fromJson(Map<String, dynamic> json) =>
      _$ServerToolConfigFromJson(json);
}

@freezed
class ToolEnabledConfig with _$ToolEnabledConfig {
  const factory ToolEnabledConfig({
    @Default(true) bool enabled,
  }) = _ToolEnabledConfig;

  factory ToolEnabledConfig.fromJson(Map<String, dynamic> json) =>
      _$ToolEnabledConfigFromJson(json);
}
