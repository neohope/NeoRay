import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/app_config.dart';
import '../providers/providers.dart';
import '../theme/app_theme.dart';
import '../constants/constants.dart';

class ConfigPage extends ConsumerStatefulWidget {
  const ConfigPage({super.key});

  @override
  ConsumerState<ConfigPage> createState() => _ConfigPageState();
}

class _ConfigPageState extends ConsumerState<ConfigPage> {
  bool _isDark(BuildContext context) => Theme.of(context).brightness == Brightness.dark;
  final _channelKey = GlobalKey();
  final _toolKey = GlobalKey();
  late TextEditingController _apiKeyController;
  late TextEditingController _apiUrlController;
  late TextEditingController _maxTokensController;
  late TextEditingController _timeoutController;
  late TextEditingController _appIdController;
  late TextEditingController _appSecretController;

  @override
  void initState() {
    super.initState();
    final config = ref.read(appConfigProvider);
    _apiKeyController = TextEditingController(text: config.llm.apiKey);
    _apiUrlController = TextEditingController(text: config.llm.apiUrl);
    _maxTokensController = TextEditingController(text: config.llm.maxTokens.toString());
    _timeoutController = TextEditingController(text: config.llm.timeout.toString());
    _appIdController = TextEditingController(text: config.channel.appId);
    _appSecretController = TextEditingController(text: config.channel.appSecret);
  }

  @override
  void dispose() {
    _apiKeyController.dispose();
    _apiUrlController.dispose();
    _maxTokensController.dispose();
    _timeoutController.dispose();
    _appIdController.dispose();
    _appSecretController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final config = ref.watch(appConfigProvider);

    return Scaffold(
      body: Row(
        children: [
          _buildSidebar(context),
          Expanded(
            child: _buildContent(context, config),
          ),
        ],
      ),
    );
  }

  Widget _buildSidebar(BuildContext context) {
    final currentPage = ref.watch(activePageProvider);

    return Container(
      width: AppDimensions.sidebarWidth,
      color: _isDark(context) ? AppTheme.sidebarBackgroundDark : AppTheme.sidebarBackgroundLight,
      child: Column(
        children: [
          _buildSidebarHeader(context),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.all(AppDimensions.spacingLg),
              child: Column(
                children: [
                  _buildNavItem(
                    context,
                    icon: Icons.chat_outlined,
                    label: AppStrings.navBackToChat,
                    onTap: () =>
                        ref.read(activePageProvider.notifier).state = AppPage.chat,
                  ),
                  const SizedBox(height: AppDimensions.spacingMd),
                  _buildNavItem(
                    context,
                    icon: Icons.smart_toy_outlined,
                    label: AppStrings.navLLMConfig,
                    isSelected: true,
                  ),
                  const SizedBox(height: AppDimensions.spacingSm),
                  _buildNavItem(
                    context,
                    icon: Icons.forum_outlined,
                    label: AppStrings.navChannelConfig,
                    onTap: () {
                      Scrollable.ensureVisible(
                        _channelKey.currentContext!,
                        duration: const Duration(milliseconds: 300),
                        alignment: 0.0,
                      );
                    },
                  ),
                  const SizedBox(height: AppDimensions.spacingSm),
                  _buildNavItem(
                    context,
                    icon: Icons.build_outlined,
                    label: AppStrings.navToolConfig,
                    onTap: () {
                      Scrollable.ensureVisible(
                        _toolKey.currentContext!,
                        duration: const Duration(milliseconds: 300),
                        alignment: 0.0,
                      );
                    },
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
      height: AppDimensions.headerHeight,
      padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacingXl),
      child: Row(
        children: [
          Container(
            width: AppDimensions.logoContainerSize,
            height: AppDimensions.logoContainerSize,
            decoration: BoxDecoration(
              color: AppTheme.primary,
              borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
            ),
            child: const Icon(
              Icons.smart_toy,
              color: Colors.white,
              size: AppDimensions.iconSizeMedium,
            ),
          ),
          const SizedBox(width: AppDimensions.spacingMd),
          Text(
            AppStrings.appName,
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
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
      borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppDimensions.spacingMd,
            vertical: AppDimensions.spacingMd,
          ),
          decoration: BoxDecoration(
            color: isSelected
                ? (_isDark(context) ? AppTheme.selectedItemBackgroundDark : AppTheme.selectedItemBackgroundLight)
                : Colors.transparent,
            borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
          ),
          child: Row(
            children: [
              Icon(
                icon,
                color: isSelected ? AppTheme.primary : (_isDark(context) ? AppTheme.textSecondaryDark : AppTheme.textSecondaryLight),
                size: AppDimensions.iconSizeSmall,
              ),
              const SizedBox(width: AppDimensions.spacingMd),
              Text(
                label,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w500,
                      color: isSelected
                          ? (_isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight)
                          : (_isDark(context) ? AppTheme.textSecondaryDark : AppTheme.textSecondaryLight),
                    ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, AppConfig config) {
    return Container(
      color: _isDark(context) ? AppTheme.backgroundDark : AppTheme.backgroundLight,
      child: Column(
        children: [
          _buildContentHeader(context),
          Expanded(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(AppDimensions.spacing2Xl),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildLLMSection(context, config.llm),
                  const SizedBox(height: AppDimensions.spacing2Xl),
                  _buildChannelSection(context, config.channel),
                  const SizedBox(height: AppDimensions.spacing2Xl),
                  _buildToolSection(context, config.tools),
                  const SizedBox(height: AppDimensions.spacing2Xl),
                  _buildSaveButton(context),
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
      height: AppDimensions.headerHeight,
      padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacing2Xl),
      child: Row(
        children: [
          Text(
            AppStrings.configModelSection,
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
          ),
        ],
      ),
    );
  }

  Widget _buildLLMSection(BuildContext context, LLMConfig config) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppDimensions.cardPadding),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              AppStrings.configLLMSection,
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildDropdownField(
              label: AppStrings.configProvider,
              value: config.provider,
              items: AppDefaults.availableProviders,
              onChanged: (value) {
                if (value != null) {
                  ref
                      .read(appConfigProvider.notifier)
                      .updateLLMConfig(config.copyWith(provider: value));
                }
              },
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildTextField(
              label: AppStrings.configApiKey,
              controller: _apiKeyController,
              obscureText: true,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(apiKey: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildTextField(
              label: AppStrings.configApiUrl,
              controller: _apiUrlController,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(apiUrl: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildDropdownField(
              label: AppStrings.configModel,
              value: config.model,
              items: AppDefaults.availableModels,
              onChanged: (value) {
                if (value != null) {
                  ref
                      .read(appConfigProvider.notifier)
                      .updateLLMConfig(config.copyWith(model: value));
                }
              },
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildNumberField(
              label: AppStrings.configMaxTokens,
              controller: _maxTokensController,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(maxTokens: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildSliderField(
              label: AppStrings.configTemperature,
              value: config.temperature,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateLLMConfig(
                        config.copyWith(temperature: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildNumberField(
              label: AppStrings.configTimeout,
              controller: _timeoutController,
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

  Widget _buildChannelSection(BuildContext context, ChannelConfig config) {
    return Card(
      key: _channelKey,
      child: Padding(
        padding: const EdgeInsets.all(AppDimensions.cardPadding),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              AppStrings.configChannelSection,
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildSwitchField(
              label: AppStrings.configEnableFeishu,
              value: config.enabled,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateChannelConfig(
                        config.copyWith(enabled: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildDropdownField(
              label: AppStrings.configChannelType,
              value: config.provider,
              items: AppDefaults.availableChannelProviders,
              onChanged: (value) {
                if (value != null) {
                  ref.read(appConfigProvider.notifier).updateChannelConfig(
                        config.copyWith(provider: value),
                      );
                }
              },
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildTextField(
              label: AppStrings.configAppId,
              controller: _appIdController,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateChannelConfig(
                        config.copyWith(appId: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildTextField(
              label: AppStrings.configAppSecret,
              controller: _appSecretController,
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

  Widget _buildToolSection(BuildContext context, ToolConfig config) {
    return Card(
      key: _toolKey,
      child: Padding(
        padding: const EdgeInsets.all(AppDimensions.cardPadding),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              AppStrings.configToolSection,
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: AppDimensions.spacingLg),
            _buildSwitchField(
              label: AppStrings.configShellTool,
              value: config.shellEnabled,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateToolConfig(
                        config.copyWith(shellEnabled: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingMd),
            _buildSwitchField(
              label: AppStrings.configCronTool,
              value: config.cronEnabled,
              onChanged: (value) =>
                  ref.read(appConfigProvider.notifier).updateToolConfig(
                        config.copyWith(cronEnabled: value),
                      ),
            ),
            const SizedBox(height: AppDimensions.spacingMd),
            _buildSwitchField(
              label: AppStrings.configWebTool,
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

  Widget _buildSaveButton(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.end,
      children: [
        ElevatedButton(
          onPressed: () async {
            await ref.read(appConfigProvider.notifier).persist();
            if (context.mounted) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text(AppStrings.configSaved)),
              );
            }
          },
          child: const Text(AppStrings.saveConfig),
        ),
      ],
    );
  }

  Widget _buildTextField({
    required String label,
    required TextEditingController controller,
    bool obscureText = false,
    required ValueChanged<String> onChanged,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: TextStyle(
            fontSize: AppDimensions.fontSizeSm,
            fontWeight: FontWeight.w500,
            color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
          ),
        ),
        const SizedBox(height: AppDimensions.spacingSm),
        TextField(
          controller: controller,
          onChanged: onChanged,
          obscureText: obscureText,
          decoration: InputDecoration(
            border: const OutlineInputBorder(),
            enabledBorder: OutlineInputBorder(
              borderSide: BorderSide(color: _isDark(context) ? AppTheme.borderDark : AppTheme.borderLight),
            ),
            focusedBorder: const OutlineInputBorder(
              borderSide: BorderSide(color: AppTheme.primary),
            ),
            contentPadding: const EdgeInsets.symmetric(
              horizontal: AppDimensions.inputHorizontalPadding,
              vertical: AppDimensions.inputVerticalPadding,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildNumberField({
    required String label,
    required TextEditingController controller,
    required ValueChanged<int> onChanged,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: TextStyle(
            fontSize: AppDimensions.fontSizeSm,
            fontWeight: FontWeight.w500,
            color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
          ),
        ),
        const SizedBox(height: AppDimensions.spacingSm),
        TextField(
          controller: controller,
          keyboardType: TextInputType.number,
          onChanged: (value) {
            final parsed = int.tryParse(value);
            if (parsed != null && parsed > 0) {
              onChanged(parsed);
            }
          },
          decoration: InputDecoration(
            border: const OutlineInputBorder(),
            enabledBorder: OutlineInputBorder(
              borderSide: BorderSide(color: _isDark(context) ? AppTheme.borderDark : AppTheme.borderLight),
            ),
            focusedBorder: const OutlineInputBorder(
              borderSide: BorderSide(color: AppTheme.primary),
            ),
            contentPadding: const EdgeInsets.symmetric(
              horizontal: AppDimensions.inputHorizontalPadding,
              vertical: AppDimensions.inputVerticalPadding,
            ),
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
          style: TextStyle(
            fontSize: AppDimensions.fontSizeSm,
            fontWeight: FontWeight.w500,
            color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
          ),
        ),
        const SizedBox(height: AppDimensions.spacingSm),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: AppDimensions.inputHorizontalPadding),
          decoration: BoxDecoration(
            color: _isDark(context) ? AppTheme.inputBackgroundDark : AppTheme.inputBackgroundLight,
            border: Border.all(color: _isDark(context) ? AppTheme.borderDark : AppTheme.borderLight),
            borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
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
          style: TextStyle(
            fontSize: AppDimensions.fontSizeSm,
            fontWeight: FontWeight.w500,
            color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
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
              style: TextStyle(
                fontSize: AppDimensions.fontSizeSm,
                fontWeight: FontWeight.w500,
                color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
              ),
            ),
            Text(
              value.toStringAsFixed(1),
              style: const TextStyle(
                fontSize: AppDimensions.fontSizeSm,
                fontWeight: FontWeight.w500,
                color: AppTheme.primary,
              ),
            ),
          ],
        ),
        const SizedBox(height: AppDimensions.spacingSm),
        Slider(
          value: value,
          min: AppDimensions.sliderMin,
          max: AppDimensions.sliderMax,
          divisions: AppDimensions.sliderDivisions,
          onChanged: onChanged,
        ),
      ],
    );
  }
}
