/// 应用字符串常量
class AppStrings {
  // 应用名称
  static const String appName = 'NeoRay';

  // 默认会话标题
  static const String defaultSessionTitle = '新聊天';
  static const String noMessagesPreview = '暂无消息';

  // UI 文本
  static const String startNewChat = '开始新对话';
  static const String sendFirstMessage = '向 NeoRay 发送第一条消息';
  static const String inputMessageHint = '输入消息...';
  static const String waitingResponse = '等待响应中...';
  static const String sendButtonLabel = '发送消息';
  static const String configSaved = '配置已保存';
  static const String saveConfig = '保存配置';

  // 导航文本
  static const String navBackToChat = '返回聊天';
  static const String navLLMConfig = '大模型配置';
  static const String navChannelConfig = 'Channel配置';
  static const String navToolConfig = '工具配置';

  // 配置标签
  static const String configModelSection = '模型配置';
  static const String configLLMSection = '大模型配置';
  static const String configChannelSection = 'Channel配置';
  static const String configToolSection = '工具配置';
  static const String configProvider = '服务商';
  static const String configApiKey = 'API Key';
  static const String configApiUrl = 'API URL';
  static const String configModel = 'Model';
  static const String configMaxTokens = 'Max Tokens';
  static const String configTemperature = 'Temperature';
  static const String configTimeout = 'Timeout (秒)';
  static const String configEnableFeishu = '启用飞书';
  static const String configChannelType = 'Channel类型';
  static const String configAppId = 'App ID';
  static const String configAppSecret = 'App Secret';
  static const String configShellTool = 'Shell工具';
  static const String configCronTool = 'Cron定时任务';
  static const String configWebTool = 'Web工具';

  // 历史记录
  static const String historyLabel = '历史聊天';
  static const String noChatHistory = '暂无聊天记录';

  // 设置
  static const String settingsLabel = '配置';

  // 通用
  static const String cancel = '取消';
  static const String confirm = '确定';
  static const String renameSession = '重命名会话';

  // 错误消息
  static const String sendFailed = '发送失败';
  static const String createSessionFailed = '创建会话失败';
  static const String loadSessionFailed = '加载会话失败';
  static const String networkError = '无法连接到服务器，请检查网络设置';
  static const String requestTimeout = '请求超时';
  static const String responseFormatError = '服务器响应格式错误';

  // 状态文本
  static const String httpWarning = '⚠️ 当前使用 HTTP 明文连接，数据未加密传输。建议在生产环境使用 HTTPS。';
  static const String thinking = '思考过程';
  static const String retry = '重试';
  static const String loading = '加载中...';
  static const String loadFailed = '加载失败';

  // 注意: channelId/userId 由 providers 在运行时通过 uuid 生成，
  // 生产环境应通过认证系统替换为真实用户/频道标识
}
