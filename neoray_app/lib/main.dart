import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'providers/providers.dart';
import 'pages/chat_page.dart';
import 'pages/config_page.dart';
import 'theme/app_theme.dart';
import 'constants/constants.dart';
import 'services/websocket_service.dart';
import 'utils/logger.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 暂时禁用 Hive，简化构建
  // await Hive.initFlutter();
  // Hive.registerAdapter(SessionAdapter());
  // Hive.registerAdapter(MessageAdapter());
  // Hive.registerAdapter(LLMConfigAdapter());
  // Hive.registerAdapter(ChannelConfigAdapter());
  // Hive.registerAdapter(ToolConfigAdapter());
  // Hive.registerAdapter(AppConfigAdapter());

  // await Hive.openBox<Session>('sessions');
  // await Hive.openBox<AppConfig>('config');

  runApp(const ProviderScope(child: MyApp()));
}

class MyApp extends ConsumerStatefulWidget {
  const MyApp({super.key});

  @override
  ConsumerState<MyApp> createState() => _MyAppState();
}

class _MyAppState extends ConsumerState<MyApp> {
  StreamSubscription<WebSocketEvent>? _eventSubscription;

  @override
  void initState() {
    super.initState();
    _setupWebSocketListener();
  }

  void _setupWebSocketListener() {
    final webSocket = ref.read(webSocketServiceProvider);
    final config = ref.read(appConfigProvider);

    // 连接到 WebSocket
    webSocket.connect(config.serverUrl);

    // 监听事件
    _eventSubscription = webSocket.eventStream.listen((event) {
      _handleWebSocketEvent(event, ref);
    });
  }

  @override
  void dispose() {
    _eventSubscription?.cancel();
    super.dispose();
  }

  void _handleWebSocketEvent(WebSocketEvent event, WidgetRef ref) {
    final currentSession = ref.read(currentSessionProvider.notifier);
    final isStreaming = ref.read(chatStreamingProvider.notifier);
    final streamingContent = ref.read(currentStreamingContentProvider.notifier);
    final isReasoning = ref.read(reasoningStreamingProvider.notifier);
    final reasoningContent = ref.read(currentReasoningContentProvider.notifier);

    switch (event.type) {
      case WebSocketMessageType.chatStart:
        isStreaming.state = true;
        streamingContent.state = '';
        break;

      case WebSocketMessageType.chatChunk:
        final chunk = event.data['content'] as String? ?? '';
        streamingContent.state += chunk;
        currentSession.addStreamingChunk(chunk);
        break;

      case WebSocketMessageType.chatEnd:
        isStreaming.state = false;
        streamingContent.state = '';
        break;

      case WebSocketMessageType.reasoningStart:
        isReasoning.state = true;
        reasoningContent.state = '';
        break;

      case WebSocketMessageType.reasoningChunk:
        final chunk = event.data['content'] as String? ?? '';
        reasoningContent.state += chunk;
        currentSession.addReasoningChunk(chunk);
        break;

      case WebSocketMessageType.reasoningEnd:
        isReasoning.state = false;
        reasoningContent.state = '';
        currentSession.completeReasoning();
        break;

      case WebSocketMessageType.error:
        isStreaming.state = false;
        final errorMsg = event.data['message'] as String? ??
            event.data['error'] as String? ??
            'An unknown error occurred';
        logger.e('Server error: $errorMsg');
        streamingContent.state = '';
        break;

      case WebSocketMessageType.toolCallStart:
      case WebSocketMessageType.toolCallResult:
      case WebSocketMessageType.sessionCreated:
      case WebSocketMessageType.sessionJoined:
      case WebSocketMessageType.sessionList:
      case WebSocketMessageType.progress:
      case WebSocketMessageType.unknown:
        // Handled elsewhere or not actionable here
        break;
    }
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: AppStrings.appName,
      debugShowCheckedModeBanner: false,
      theme: AppTheme.lightTheme,
      darkTheme: AppTheme.darkTheme,
      themeMode: ThemeMode.system,
      home: const MainScreen(),
    );
  }
}

class MainScreen extends ConsumerWidget {
  const MainScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final activePage = ref.watch(activePageProvider);

    return Scaffold(
      body: IndexedStack(
        index: activePage.index,
        children: const [
          ChatPage(),
          ConfigPage(),
        ],
      ),
    );
  }
}
