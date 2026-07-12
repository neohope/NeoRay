import 'package:freezed_annotation/freezed_annotation.dart';
import '../constants/constants.dart';

part 'app_config.freezed.dart';
part 'app_config.g.dart';

@freezed
class LLMConfig with _$LLMConfig {
  const factory LLMConfig({
    @Default(AppDefaults.defaultLLMProvider) String provider,
    @Default('') String apiKey,
    @Default(AppDefaults.defaultApiUrl) String apiUrl,
    @Default(AppDefaults.defaultLLMModel) String model,
    @Default(AppDefaults.defaultMaxTokens) int maxTokens,
    @Default(AppDefaults.defaultTemperature) double temperature,
    @Default(AppDefaults.defaultTimeoutSec) int timeout,
    @Default(AppDefaults.defaultReasoningEffort) String reasoningEffort,
    @Default(AppDefaults.defaultPromptCacheEnabled) bool promptCacheEnabled,
    @Default([]) List<FallbackModelConfig> fallbackModels,
  }) = _LLMConfig;

  factory LLMConfig.fromJson(Map<String, dynamic> json) =>
      _$LLMConfigFromJson(json);
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
class ChannelConfig with _$ChannelConfig {
  const factory ChannelConfig({
    @Default(AppDefaults.defaultChannelEnabled) bool enabled,
    @Default(AppDefaults.defaultChannelProvider) String provider,
    @Default('') String appId,
    @Default('') String appSecret,
  }) = _ChannelConfig;

  factory ChannelConfig.fromJson(Map<String, dynamic> json) =>
      _$ChannelConfigFromJson(json);
}

@freezed
class ToolConfig with _$ToolConfig {
  const factory ToolConfig({
    @Default(AppDefaults.defaultShellEnabled) bool shellEnabled,
    @Default(AppDefaults.defaultCronEnabled) bool cronEnabled,
    @Default(AppDefaults.defaultWebEnabled) bool webEnabled,
  }) = _ToolConfig;

  factory ToolConfig.fromJson(Map<String, dynamic> json) =>
      _$ToolConfigFromJson(json);
}

@freezed
class AppConfig with _$AppConfig {
  const factory AppConfig({
    @Default(AppDefaults.defaultServerUrl) String serverUrl,
    @Default(LLMConfig()) LLMConfig llm,
    @Default(ChannelConfig()) ChannelConfig channel,
    @Default(ToolConfig()) ToolConfig tools,
  }) = _AppConfig;

  factory AppConfig.fromJson(Map<String, dynamic> json) =>
      _$AppConfigFromJson(json);
}
