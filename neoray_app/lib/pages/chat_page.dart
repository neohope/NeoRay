import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/session.dart';
import '../models/message.dart';
import '../providers/providers.dart';
import '../theme/app_theme.dart';
import '../utils/logger.dart';
import 'config_page.dart';
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
            content: Text('发送失败: $e'),
            backgroundColor: AppTheme.danger,
            duration: const Duration(seconds: 4),
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
    Future.delayed(const Duration(milliseconds: 100), () {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
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
            content: Text('创建会话失败: $e'),
            backgroundColor: AppTheme.danger,
            duration: const Duration(seconds: 3),
          ),
        );
      }
    }
  }

  void _openSettings() {
    ref.read(activePageProvider.notifier).state = AppPage.config;
  }

  @override
  Widget build(BuildContext context) {
    final currentSession = ref.watch(currentSessionProvider);
    final isStreaming = ref.watch(chatStreamingProvider);
    final streamingContent = ref.watch(currentStreamingContentProvider);
    final colorScheme = Theme.of(context).colorScheme;

    return Scaffold(
      body: Row(
        children: [
          Sidebar(
            onNewChat: _createNewChat,
            onOpenSettings: _openSettings,
          ),
          Expanded(
            child: Container(
              color: AppTheme.backgroundLight,
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
      height: 70,
      padding: const EdgeInsets.symmetric(horizontal: 24),
      child: Row(
        children: [
          Text(
            currentSession?.title ?? '新聊天',
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
          ),
          const Spacer(),
          if (currentSession != null)
            IconButton(
              icon: const Icon(Icons.edit_outlined),
              onPressed: () {},
            ),
        ],
      ),
    );
  }

  Widget _buildWelcomeScreen(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.chat_outlined,
              size: 80,
              color: colorScheme.primary.withValues(alpha: 0.3),
            ),
            const SizedBox(height: 24),
            Text(
              '开始新对话',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    color: AppTheme.textPrimaryLight,
                  ),
            ),
            const SizedBox(height: 8),
            Text(
              '向 NeoRay 发送第一条消息',
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

    if (!hasMessages && !isStreaming) {
      return _buildWelcomeScreen(context);
    }

    return ListView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 24),
      itemCount: session.messages.length + (isStreaming ? 1 : 0),
      itemBuilder: (context, index) {
        if (index == session.messages.length && isStreaming) {
          return MessageBubble(
            message: Message.assistant(streamingContent),
            isStreaming: true,
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
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      color: Theme.of(context).cardColor,
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
      child: SafeArea(
        child: Row(
          children: [
            Expanded(
              child: TextField(
                controller: _messageController,
                decoration: const InputDecoration(
                  hintText: '输入消息...',
                  border: OutlineInputBorder(),
                  enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.all(Radius.circular(12)),
                    borderSide: BorderSide(color: Color(0xFFE5E7EB)),
                  ),
                  focusedBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.all(Radius.circular(12)),
                    borderSide: BorderSide(color: AppTheme.primary),
                  ),
                ),
                maxLines: null,
                onSubmitted: (_) => _sendMessage(),
              ),
            ),
            const SizedBox(width: 12),
            Container(
              decoration: BoxDecoration(
                color: AppTheme.primary,
                borderRadius: BorderRadius.circular(8),
              ),
              child: IconButton(
                icon: const Icon(
                  Icons.send_rounded,
                  color: Colors.white,
                ),
                onPressed: _sendMessage,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
