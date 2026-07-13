import 'package:flutter/foundation.dart';
import 'package:logger/logger.dart';

final logger = Logger(
  level: kReleaseMode ? Level.warning : Level.debug,
  printer: PrettyPrinter(
    methodCount: 0,
    colors: !kReleaseMode,
    lineLength: 80,
    printEmojis: !kReleaseMode,
    dateTimeFormat: DateTimeFormat.dateAndTime,
  ),
);
