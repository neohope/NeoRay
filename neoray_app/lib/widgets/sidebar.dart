import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/session.dart';
import '../providers/providers.dart';
import '../theme/app_theme.dart';
import '../utils/logger.dart';

class Sidebar extends ConsumerWidget {
  final VoidCallback onNewChat;
  final VoidCallback onOpenSettings;

  const Sidebar({
    super.key,
    required this.onNewChat,
    required this.onOpenSettings,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionList = ref.watch(sessionListProvider);
    final currentSession = ref.watch(currentSessionProvider);

    return Container(
      width: 280,
      color: const Color(0xFFF0F2F5),
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
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.error_outline,
                        size: 48,
                        color: AppTheme.danger.withValues(alpha: 0.6),
                      ),
                      const SizedBox(height: 16),
                      Text(
                        '加载失败',
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        error.toString(),
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                              color: AppTheme.textSecondaryLight,
                            ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 16),
                      ElevatedButton.icon(
                        onPressed: () {
                          ref.read(sessionListProvider.notifier).loadSessions();
                        },
                        icon: const Icon(Icons.refresh, color: Colors.white),
                        label: const Text('重试', style: TextStyle(color: Colors.white)),
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

  Widget _buildNewChatButton(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: ElevatedButton.icon(
        onPressed: onNewChat,
        icon: const Icon(Icons.add, color: Colors.white),
        label: const Text('新聊天', style: TextStyle(color: Colors.white)),
        style: ElevatedButton.styleFrom(
          backgroundColor: AppTheme.primary,
          minimumSize: const Size(double.infinity, 44),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(10),
          ),
          elevation: 0,
        ),
      ),
    );
  }

  Widget _buildHistoryLabel(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: 20, right: 20, top: 12, bottom: 8),
      child: Align(
        alignment: Alignment.centerLeft,
        child: Text(
          '历史聊天',
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
          '暂无聊天记录',
          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: AppTheme.textSecondaryLight,
              ),
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      itemCount: sessions.length,
      itemBuilder: (context, index) {
        final session = sessions[index];
        final isSelected = currentSession?.id == session.id;

        return Padding(
          padding: const EdgeInsets.only(bottom: 8),
          child: Material(
            color: isSelected ? const Color(0xFFE5E7EB) : Colors.transparent,
            borderRadius: BorderRadius.circular(8),
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
                        content: Text('加载会话失败: $e'),
                        backgroundColor: AppTheme.danger,
                        duration: const Duration(seconds: 3),
                      ),
                    );
                  }
                }
              },
              borderRadius: BorderRadius.circular(8),
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Row(
                  children: [
                    Icon(
                      Icons.chat_outlined,
                      size: 18,
                      color: isSelected
                          ? AppTheme.primary
                          : AppTheme.textSecondaryLight,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            session.title ?? '新聊天',
                            style: Theme.of(context)
                                .textTheme
                                .bodyMedium
                                ?.copyWith(
                                  fontWeight: FontWeight.w500,
                                  color: AppTheme.textPrimaryLight,
                                ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          const SizedBox(height: 2),
                          Text(
                            _getPreview(session),
                            style:
                                Theme.of(context).textTheme.bodySmall?.copyWith(
                                      color: AppTheme.textSecondaryLight,
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
    if (session.messages.isEmpty) return '暂无消息';
    final lastMessage = session.messages.last;
    return lastMessage.content;
  }

  Widget _buildFooter(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(8),
        child: InkWell(
          onTap: onOpenSettings,
          borderRadius: BorderRadius.circular(8),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
            child: Row(
              children: [
                const Icon(
                  Icons.settings_outlined,
                  size: 18,
                  color: AppTheme.textSecondaryLight,
                ),
                const SizedBox(width: 12),
                Text(
                  '配置',
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        fontWeight: FontWeight.w500,
                        color: AppTheme.textPrimaryLight,
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
