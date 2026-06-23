import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'providers/providers.dart';
import 'pages/chat_page.dart';
import 'pages/config_page.dart';
import 'theme/app_theme.dart';

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
  @override
  void initState() {
    super.initState();
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'NeoRay',
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
