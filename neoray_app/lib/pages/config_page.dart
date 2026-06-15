import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/app_config.dart';
import '../providers/providers.dart';
import '../theme/app_theme.dart';

class ConfigPage extends ConsumerWidget {
  const ConfigPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final config = ref.watch(appConfigProvider);

    return Scaffold(
      body: Row(
        children: [
          _buildSidebar(context, ref),
          Expanded(
            child: _buildContent(context, ref, config),
          ),
        ],
      ),
    );
  }

  Widget _buildSidebar(BuildContext context, WidgetRef ref) {
    final currentPage = ref.watch(activePageProvider);

    return Container(
      width: 280,
      color: const Color(0xFFF0F2F5),
      child: Column(
        children: [
          _buildSidebarHeader(context),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                children: [
                  _buildNavItem(
                    context,
                    icon: Icons.chat_outlined,
                    label: '返回聊天',
                    onTap: () =>
                        ref.read(activePageProvider.notifier).state = AppPage.chat,
                  ),
                  const SizedBox(height: 12),
                  _buildNavItem(
                    context,
                    icon: Icons.smart_toy_outlined,
                    label: '大模型配置',
                    isSelected: true,
                  ),
                  const SizedBox(height: 8),
                  _buildNavItem(
                    context,
                    icon: Icons.forum_outlined,
                    label: 'Channel配置',
                  ),
                  const SizedBox(height: 8),
                  _buildNavItem(
                    context,
                    icon: Icons.build_outlined,
                    label: '工具配置',
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSidebarHeader(BuildContext context) {
    return Container(
      height: 70,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Row(
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: BoxDecoration(
              color: AppTheme.primary,
              borderRadius: BorderRadius.circular(8),
            ),
            child: const Icon(
              Icons.smart_toy,
              color: Colors.white,
              size: 20,
            ),
          ),
          const SizedBox(width: 12),
          Text(
            'NeoRay',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: AppTheme.textPrimaryLight,
                ),
          ),
        ],
      ),
    );
  }

  Widget _buildNavItem(
    BuildContext context, {
    required IconData icon,
    required String label,
    VoidCallback? onTap,
    bool isSelected = false,
  }) {
    return Material(
      color: Colors.transparent,
      borderRadius: BorderRadius.circular(8),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
          decoration: BoxDecoration(
            color: isSelected ? const Color(0xFFE5E7EB) : Colors.transparent,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            children: [
              Icon(
                icon,
                color: isSelected ? AppTheme.primary : AppTheme.textSecondaryLight,
                size: 18,
              ),
              const SizedBox(width: 12),
              Text(
                label,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w500,
                      color: isSelected ? AppTheme.textPrimaryLight : AppTheme.textSecondaryLight,
                    ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, WidgetRef ref, AppConfig config) {
    return Container(
      color: AppTheme.backgroundLight,
      child: Column(
        children: [
          _buildContentHeader(context),
          Expanded(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildLLMSection(context, ref, config.llm),
                  const SizedBox(height: 24),
                  _buildChannelSection(context, ref, config.channel),
                  const SizedBox(height: 24),
                  _buildToolSection(context, ref, config.tools),
                  const SizedBox(height: 24),
                  _buildSaveButton(context, ref),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildContentHeader(BuildContext context) {
    return Container(
      color: Theme.of(context).cardColor,
      height: 70,
      padding: const EdgeInsets.symmetric(horizontal: 24),
      child: Row(
        children: [
          Text(
            '模型配置',
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
          ),
        ],
      ),
    );
  }

  Widget _buildLLMSection(BuildContext context, WidgetRef ref, LLMConfig config) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '大模型配置',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            _buildDropdownField(
              label: '服务商',
              value: config.provider,
              items: const [
                'openai',
                'anthropic',
              ],
              onChanged: (value) {
                if (value != null) {
                  ref
                      .read(appConfigProvider.notifier)
                      .updateLLMConfig(config.copyWith(provider: value));
                }
              },
            ),
            const SizedBox(height: 16),
            _buildTextField(
              label: 'API Key',
              value: config.apiKey,
              obscureText: true,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(apiKey: value),
                      ),
            ),
            const SizedBox(height: 16),
            _buildTextField(
              label: 'API URL',
              value: config.apiUrl,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(apiUrl: value),
                      ),
            ),
            const SizedBox(height: 16),
            _buildDropdownField(
              label: 'Model',
              value: config.model,
              items: const [
                'gpt-4',
                'gpt-3.5-turbo',
                'claude-3-opus',
                'claude-3-sonnet',
              ],
              onChanged: (value) {
                if (value != null) {
                  ref
                      .read(appConfigProvider.notifier)
                      .updateLLMConfig(config.copyWith(model: value));
                }
              },
            ),
            const SizedBox(height: 16),
            _buildNumberField(
              label: 'Max Tokens',
              value: config.maxTokens,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(maxTokens: value),
                      ),
            ),
            const SizedBox(height: 16),
            _buildSliderField(
              label: 'Temperature',
              value: config.temperature,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(temperature: value),
                      ),
            ),
            const SizedBox(height: 16),
            _buildNumberField(
              label: 'Timeout (秒)',
              value: config.timeout,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(timeout: value),
                      ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildChannelSection(BuildContext context, WidgetRef ref, ChannelConfig config) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Channel配置',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            _buildSwitchField(
              label: '启用飞书',
              value: config.enabled,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateChannelConfig(
                        config.copyWith(enabled: value),
                      ),
            ),
            const SizedBox(height: 16),
            _buildDropdownField(
              label: 'Channel类型',
              value: config.provider,
              items: const ['feishu'],
              onChanged: (value) {
                if (value != null) {
                  ref.read(appConfigProvider.notifier).updateChannelConfig(
                        config.copyWith(provider: value),
                      );
                }
              },
            ),
            const SizedBox(height: 16),
            _buildTextField(
              label: 'App ID',
              value: config.appId,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateChannelConfig(
                        config.copyWith(appId: value),
                      ),
            ),
            const SizedBox(height: 16),
            _buildTextField(
              label: 'App Secret',
              value: config.appSecret,
              obscureText: true,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateChannelConfig(
                        config.copyWith(appSecret: value),
                      ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildToolSection(BuildContext context, WidgetRef ref, ToolConfig config) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '工具配置',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            _buildSwitchField(
              label: 'Shell工具',
              value: config.shellEnabled,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateToolConfig(
                        config.copyWith(shellEnabled: value),
                      ),
            ),
            const SizedBox(height: 12),
            _buildSwitchField(
              label: 'Cron定时任务',
              value: config.cronEnabled,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateToolConfig(
                        config.copyWith(cronEnabled: value),
                      ),
            ),
            const SizedBox(height: 12),
            _buildSwitchField(
              label: 'Web工具',
              value: config.webEnabled,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateToolConfig(
                        config.copyWith(webEnabled: value),
                      ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSaveButton(BuildContext context, WidgetRef ref) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.end,
      children: [
        ElevatedButton(
          onPressed: () {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('配置已保存')),
            );
          },
          child: const Text('保存配置'),
        ),
      ],
    );
  }

  Widget _buildTextField({
    required String label,
    required String value,
    bool obscureText = false,
    required ValueChanged<String> onChanged,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
            color: AppTheme.textPrimaryLight,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: TextEditingController(text: value),
          onChanged: onChanged,
          obscureText: obscureText,
          decoration: const InputDecoration(
            border: OutlineInputBorder(),
            enabledBorder: OutlineInputBorder(
              borderSide: BorderSide(color: Color(0xFFE5E7EB)),
            ),
            focusedBorder: OutlineInputBorder(
              borderSide: BorderSide(color: AppTheme.primary),
            ),
            contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 14),
          ),
        ),
      ],
    );
  }

  Widget _buildNumberField({
    required String label,
    required int value,
    required ValueChanged<int> onChanged,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
            color: AppTheme.textPrimaryLight,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: TextEditingController(text: value.toString()),
          keyboardType: TextInputType.number,
          onChanged: (value) {
            onChanged(int.tryParse(value) ?? 0);
          },
          decoration: const InputDecoration(
            border: OutlineInputBorder(),
            enabledBorder: OutlineInputBorder(
              borderSide: BorderSide(color: Color(0xFFE5E7EB)),
            ),
            focusedBorder: OutlineInputBorder(
              borderSide: BorderSide(color: AppTheme.primary),
            ),
            contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 14),
          ),
        ),
      ],
    );
  }

  Widget _buildDropdownField({
    required String label,
    required String value,
    required List<String> items,
    required ValueChanged<String?> onChanged,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
            color: AppTheme.textPrimaryLight,
          ),
        ),
        const SizedBox(height: 8),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          decoration: BoxDecoration(
            color: const Color(0xFFF9FAFB),
            border: Border.all(color: const Color(0xFFE5E7EB)),
            borderRadius: BorderRadius.circular(8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<String>(
              value: value,
              isExpanded: true,
              items: items.map((item) {
                return DropdownMenuItem(
                  value: item,
                  child: Text(item),
                );
              }).toList(),
              onChanged: onChanged,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildSwitchField({
    required String label,
    required bool value,
    required ValueChanged<bool> onChanged,
  }) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w500,
            color: AppTheme.textPrimaryLight,
          ),
        ),
        Switch(
          value: value,
          onChanged: onChanged,
        ),
      ],
    );
  }

  Widget _buildSliderField({
    required String label,
    required double value,
    required ValueChanged<double> onChanged,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              label,
              style: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w500,
                color: AppTheme.textPrimaryLight,
              ),
            ),
            Text(
              value.toStringAsFixed(1),
              style: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w500,
                color: AppTheme.primary,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Slider(
          value: value,
          min: 0,
          max: 2,
          divisions: 20,
          onChanged: onChanged,
        ),
      ],
    );
  }
}
