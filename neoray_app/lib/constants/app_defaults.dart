/// 应用默认配置值
class AppDefaults {
  // 服务器配置
  static const String defaultServerUrl = 'http://localhost:8080';

  // LLM 配置
  static const String defaultLLMProvider = 'openai';
  static const String defaultLLMModel = 'gpt-4o';
  static const String defaultApiUrl = 'https://api.openai.com/v1';
  static const int defaultMaxTokens = 4096;
  static const double defaultTemperature = 0.7;
  static const double defaultTimeoutSec = 120.0;
  static const String defaultReasoningEffort = 'none';
  static const bool defaultPromptCacheEnabled = false;

  // 可用模型列表
  static const List<String> availableModels = [
    'gpt-4o',
    'gpt-4o-mini',
    'claude-sonnet-4-20250514',
    'claude-haiku-4-5-20251001',
  ];

  // 可用提供商列表
  static const List<String> availableProviders = [
    'openai',
    'anthropic',
  ];

  // Channel 配置
  static const bool defaultChannelEnabled = false;
  static const String defaultChannelProvider = 'feishu';
  static const List<String> availableChannelProviders = [
    'feishu',
  ];

  // 工具配置
  static const bool defaultShellEnabled = true;
  static const bool defaultCronEnabled = true;
  static const bool defaultWebEnabled = false;

  // 主题配置: 'light' | 'dark' | 'system'
  static const String defaultThemeMode = 'light';
}
