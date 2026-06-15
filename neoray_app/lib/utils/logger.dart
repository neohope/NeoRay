import 'package:logger/logger.dart';

final logger = Logger(
  printer: PrettyPrinter(
    methodCount: 0,
    colors: true,
    printTime: true,
    printEmojis: true,
  ),
);
