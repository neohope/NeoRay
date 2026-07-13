import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/session.dart';
import '../providers/providers.dart';
import '../theme/app_theme.dart';
import '../constants/constants.dart';

class Sidebar extends ConsumerWidget {
  final VoidCallback onNewChat;
  final VoidCallback onOpenSettings;

  const Sidebar({
    super.key,
    required this.onNewChat,
    required this.onOpenSettings,
  });

  bool _isDark(BuildContext context) => Theme.of(context).brightness == Brightness.dark;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionList = ref.watch(sessionListProvider);
    final currentSession = ref.watch(currentSessionProvider);

    return Container(
      width: AppDimensions.sidebarWidth,
      color: _isDark(context) ? AppTheme.sidebarBackgroundDark : AppTheme.sidebarBackgroundLight,
      child: Column(
        children: [
          _buildHeader(context),
          _buildNewChatButton(context),
          _buildHistoryLabel(context),
          Expanded(
            child: sessionList.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, stackTrace) => Center(
                child: Padding(
                  padding: const EdgeInsets.all(AppDimensions.spacingLg),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.error_outline,
                        size: 48,
                        color: AppTheme.dangerTransparent60,
                      ),
                      const SizedBox(height: AppDimensions.spacingLg),
                      Text(
                        AppStrings.loadFailed,
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: AppDimensions.spacingSm),
                      Text(
                        error.toString(),
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                              color: _isDark(context) ? AppTheme.textSecondaryDark : AppTheme.textSecondaryLight,
                            ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: AppDimensions.spacingLg),
                      ElevatedButton.icon(
                        onPressed: () {
                          ref.read(sessionListProvider.notifier).loadSessions();
                        },
                        icon: const Icon(Icons.refresh, color: Colors.white),
                        label: const Text(AppStrings.retry,
                            style: TextStyle(color: Colors.white)),
                        style: ElevatedButton.styleFrom(
                          backgroundColor: AppTheme.primary,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              data: (sessions) => _buildSessionList(
                context,
                ref,
                sessions,
                currentSession,
              ),
            ),
          ),
          _buildFooter(context),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
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

  Widget _buildNewChatButton(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacingLg),
      child: ElevatedButton.icon(
        onPressed: onNewChat,
        icon: const Icon(Icons.add, color: Colors.white),
        label: const Text(AppStrings.defaultSessionTitle, style: TextStyle(color: Colors.white)),
        style: ElevatedButton.styleFrom(
          backgroundColor: AppTheme.primary,
          minimumSize: const Size(double.infinity, AppDimensions.buttonMinHeight),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppDimensions.borderRadiusMd),
          ),
          elevation: 0,
        ),
      ),
    );
  }

  Widget _buildHistoryLabel(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(
        left: AppDimensions.spacingXl,
        right: AppDimensions.spacingXl,
        top: AppDimensions.spacingMd,
        bottom: AppDimensions.spacingSm,
      ),
      child: Align(
        alignment: Alignment.centerLeft,
        child: Text(
          AppStrings.historyLabel,
          style: Theme.of(context).textTheme.labelLarge?.copyWith(
                color: AppTheme.textSecondaryLight,
                fontWeight: FontWeight.w600,
              ),
        ),
      ),
    );
  }

  Widget _buildSessionList(
    BuildContext context,
    WidgetRef ref,
    List<Session> sessions,
    Session? currentSession,
  ) {
    if (sessions.isEmpty) {
      return Center(
        child: Text(
          AppStrings.noChatHistory,
          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: AppTheme.textSecondaryLight,
              ),
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacingLg),
      itemCount: sessions.length,
      itemBuilder: (context, index) {
        final session = sessions[index];
        final isSelected = currentSession?.id == session.id;

        return Padding(
          padding: const EdgeInsets.only(bottom: AppDimensions.spacingSm),
          child: Material(
            color: isSelected ? (_isDark(context) ? AppTheme.selectedItemBackgroundDark : AppTheme.selectedItemBackgroundLight) : Colors.transparent,
            borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
            child: InkWell(
              onTap: () async {
                try {
                  await ref
                      .read(currentSessionProvider.notifier)
                      .selectSession(session.id);
                } catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text('${AppStrings.loadSessionFailed}: $e'),
                        backgroundColor: AppTheme.danger,
                        duration: const Duration(seconds: AppTimings.snackBarDurationShortSec),
                      ),
                    );
                  }
                }
              },
              borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
              child: Padding(
                padding: const EdgeInsets.all(AppDimensions.spacingMd),
                child: Row(
                  children: [
                    Icon(
                      Icons.chat_outlined,
                      size: AppDimensions.iconSizeSmall,
                      color: isSelected
                          ? AppTheme.primary
                          : AppTheme.textSecondaryLight,
                    ),
                    const SizedBox(width: AppDimensions.spacingMd),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            session.title ?? AppStrings.defaultSessionTitle,
                            style: Theme.of(context)
                                .textTheme
                                .bodyMedium
                                ?.copyWith(
                                  fontWeight: FontWeight.w500,
                                  color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
                                ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          const SizedBox(height: AppDimensions.spacingXs),
                          Text(
                            _getPreview(session),
                            style:
                                Theme.of(context).textTheme.bodySmall?.copyWith(
                                      color: _isDark(context) ? AppTheme.textSecondaryDark : AppTheme.textSecondaryLight,
                                    ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  String _getPreview(Session session) {
    if (session.messages.isEmpty) return AppStrings.noMessagesPreview;
    final lastMessage = session.messages.last;
    return lastMessage.content;
  }

  Widget _buildFooter(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(AppDimensions.spacingLg),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
        child: InkWell(
          onTap: onOpenSettings,
          borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppDimensions.spacingMd,
              vertical: AppDimensions.spacingMd,
            ),
            child: Row(
              children: [
                Icon(
                  Icons.settings_outlined,
                  size: AppDimensions.iconSizeSmall,
                  color: _isDark(context) ? AppTheme.textSecondaryDark : AppTheme.textSecondaryLight,
                ),
                const SizedBox(width: AppDimensions.spacingMd),
                Text(
                  AppStrings.settingsLabel,
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        fontWeight: FontWeight.w500,
                        color: _isDark(context) ? AppTheme.textPrimaryDark : AppTheme.textPrimaryLight,
                      ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
