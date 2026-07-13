import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/session.dart';
import '../models/message.dart';
import '../providers/providers.dart';
import '../theme/app_theme.dart';
import '../constants/constants.dart';
import '../utils/logger.dart';
import '../widgets/message_bubble.dart';
import '../widgets/sidebar.dart';

class ChatPage extends ConsumerStatefulWidget {
  const ChatPage({super.key});

  @override
  ConsumerState<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends ConsumerState<ChatPage> {
  final TextEditingController _messageController = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  bool _isTyping = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadSessions();
    });
  }

  void _loadSessions() {
    ref.read(sessionListProvider.notifier).loadSessions();
  }

  @override
  void dispose() {
    _messageController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _sendMessage() async {
    final message = _messageController.text.trim();
    if (message.isEmpty) return;

    final currentSession = ref.read(currentSessionProvider);
    final currentSessionNotifier = ref.read(currentSessionProvider.notifier);

    if (currentSession == null) {
      await currentSessionNotifier.newSession();
    }

    _messageController.clear();
    setState(() {
      _isTyping = true;
    });

    try {
      await currentSessionNotifier.sendMessage(message);
    } catch (e) {
      logger.e('发送消息失败', error: e);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('${AppStrings.sendFailed}: $e'),
            backgroundColor: AppTheme.danger,
            duration: const Duration(seconds: AppTimings.snackBarDurationLongSec),
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _isTyping = false;
        });
      }
    }

    _scrollToBottom();
  }

  void _scrollToBottom() {
    Future.delayed(const Duration(milliseconds: AppTimings.scrollDelayMs), () {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: AppTimings.animationDurationShortMs),
          curve: Curves.easeOut,
        );
      }
    });
  }

  void _createNewChat() async {
    try {
      await ref.read(sessionListProvider.notifier).createSession();
      ref.read(currentSessionProvider.notifier).newSession();
      _messageController.clear();
    } catch (e) {
      logger.e('创建会话失败', error: e);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('${AppStrings.createSessionFailed}: $e'),
            backgroundColor: AppTheme.danger,
            duration: const Duration(seconds: AppTimings.snackBarDurationShortSec),
          ),
        );
      }
    }
  }

  void _openSettings() {
    ref.read(activePageProvider.notifier).state = AppPage.config;
  }

  void _showRenameDialog(BuildContext context, Session session) {
    final controller = TextEditingController(text: session.title ?? '');
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(AppStrings.renameSession),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(hintText: AppStrings.defaultSessionTitle),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: Text(AppStrings.cancel),
          ),
          TextButton(
            onPressed: () {
              final newTitle = controller.text.trim();
              if (newTitle.isNotEmpty) {
                ref.read(currentSessionProvider.notifier).renameTitle(newTitle);
              }
              Navigator.pop(dialogContext);
            },
            child: Text(AppStrings.confirm),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final currentSession = ref.watch(currentSessionProvider);
    final isStreaming = ref.watch(chatStreamingProvider);
    final streamingContent = ref.watch(currentStreamingContentProvider);

    // 流式回复时自动滚动到底部
    ref.listen<String>(currentStreamingContentProvider, (prev, next) {
      if (next.isNotEmpty && (prev == null || prev != next)) {
        _scrollToBottom();
      }
    });

    return Scaffold(
      body: Row(
        children: [
          Sidebar(
            onNewChat: _createNewChat,
            onOpenSettings: _openSettings,
          ),
          Expanded(
            child: Container(
              color: Theme.of(context).scaffoldBackgroundColor,
              child: Column(
                children: [
                  _buildHeader(context),
                  Expanded(
                    child: currentSession == null
                        ? _buildWelcomeScreen(context)
                        : _buildMessageList(
                            currentSession,
                            isStreaming,
                            streamingContent,
                          ),
                  ),
                  _buildInputArea(context),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    final currentSession = ref.watch(currentSessionProvider);

    return Container(
      color: Theme.of(context).cardColor,
      height: AppDimensions.headerHeight,
      padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacing2Xl),
      child: Row(
        children: [
          Text(
            currentSession?.title ?? AppStrings.defaultSessionTitle,
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
          ),
          const Spacer(),
          if (currentSession != null)
            IconButton(
              icon: const Icon(Icons.edit_outlined),
              onPressed: () => _showRenameDialog(context, currentSession),
            ),
        ],
      ),
    );
  }

  Widget _buildWelcomeScreen(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacing2Xl),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.chat_outlined,
              size: AppDimensions.iconSizeXXLarge,
              color: colorScheme.primary.withValues(alpha: 0.3),
            ),
            const SizedBox(height: AppDimensions.spacing2Xl),
            Text(
              AppStrings.startNewChat,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: AppDimensions.spacingSm),
            Text(
              AppStrings.sendFirstMessage,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildMessageList(
    Session session,
    bool isStreaming,
    String streamingContent,
  ) {
    final hasMessages = session.messages.isNotEmpty;
    final isReasoning = ref.watch(reasoningStreamingProvider);
    final reasoningContent = ref.watch(currentReasoningContentProvider);

    if (!hasMessages && !isStreaming && !isReasoning) {
      return _buildWelcomeScreen(context);
    }

    return ListView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacing2Xl, vertical: AppDimensions.spacing2Xl),
      itemCount: session.messages.length + ((isStreaming || isReasoning) ? 1 : 0),
      itemBuilder: (context, index) {
        if (index == session.messages.length && (isStreaming || isReasoning)) {
          return MessageBubble(
            message: Message.assistant(
              streamingContent,
              null,
              session.channelId,
              session.userId,
              session.id,
              isReasoning ? reasoningContent : null,
              false,
            ),
            isStreaming: isStreaming,
          );
        }

        final message = session.messages[index];
        return MessageBubble(
          message: message,
        );
      },
    );
  }

  Widget _buildInputArea(BuildContext context) {
    final isStreaming = ref.watch(chatStreamingProvider);
    final isBusy = _isTyping || isStreaming;

    return Container(
      color: Theme.of(context).cardColor,
      padding: const EdgeInsets.symmetric(horizontal: AppDimensions.spacing2Xl, vertical: AppDimensions.spacingLg),
      child: SafeArea(
        child: Row(
          children: [
            Expanded(
              child: TextField(
                controller: _messageController,
                enabled: !isBusy,
                decoration: InputDecoration(
                  hintText: isBusy ? AppStrings.waitingResponse : AppStrings.inputMessageHint,
                  border: const OutlineInputBorder(),
                  enabledBorder: const OutlineInputBorder(
                    borderRadius: BorderRadius.all(Radius.circular(AppDimensions.borderRadiusLg)),
                    borderSide: BorderSide(color: AppTheme.borderLight),
                  ),
                  focusedBorder: const OutlineInputBorder(
                    borderRadius: BorderRadius.all(Radius.circular(AppDimensions.borderRadiusLg)),
                    borderSide: BorderSide(color: AppTheme.primary),
                  ),
                ),
                maxLines: null,
                onSubmitted: isBusy ? null : (_) => _sendMessage(),
              ),
            ),
            const SizedBox(width: AppDimensions.spacingMd),
            Container(
              decoration: BoxDecoration(
                color: isBusy ? AppTheme.textSecondaryLight : AppTheme.primary,
                borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
              ),
              child: Semantics(
                label: isBusy ? AppStrings.waitingResponse : AppStrings.sendButtonLabel,
                button: true,
                child: IconButton(
                  icon: isBusy
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : const Icon(
                          Icons.send_rounded,
                          color: Colors.white,
                        ),
                  onPressed: isBusy ? null : _sendMessage,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
