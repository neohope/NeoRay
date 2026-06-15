import 'package:freezed_annotation/freezed_annotation.dart';

part 'app_config.freezed.dart';
part 'app_config.g.dart';

@freezed
class LLMConfig with _$LLMConfig {
  const factory LLMConfig({
    @Default('openai') String provider,
    @Default('') String apiKey,
    @Default('https://api.openai.com/v1') String apiUrl,
    @Default('gpt-4') String model,
    @Default(4096) int maxTokens,
    @Default(0.7) double temperature,
    @Default(120) int timeout,
  }) = _LLMConfig;

  factory LLMConfig.fromJson(Map<String, dynamic> json) =>
      _$LLMConfigFromJson(json);
}

@freezed
class ChannelConfig with _$ChannelConfig {
  const factory ChannelConfig({
    @Default(false) bool enabled,
    @Default('feishu') String provider,
    @Default('') String appId,
    @Default('') String appSecret,
  }) = _ChannelConfig;

  factory ChannelConfig.fromJson(Map<String, dynamic> json) =>
      _$ChannelConfigFromJson(json);
}

@freezed
class ToolConfig with _$ToolConfig {
  const factory ToolConfig({
    @Default(true) bool shellEnabled,
    @Default(true) bool cronEnabled,
    @Default(false) bool webEnabled,
  }) = _ToolConfig;

  factory ToolConfig.fromJson(Map<String, dynamic> json) =>
      _$ToolConfigFromJson(json);
}

@freezed
class AppConfig with _$AppConfig {
  const factory AppConfig({
    @Default('http://localhost:8080') String serverUrl,
    @Default(LLMConfig()) LLMConfig llm,
    @Default(ChannelConfig()) ChannelConfig channel,
    @Default(ToolConfig()) ToolConfig tools,
  }) = _AppConfig;

  factory AppConfig.fromJson(Map<String, dynamic> json) =>
      _$AppConfigFromJson(json);
}
