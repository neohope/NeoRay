import 'package:logger/logger.dart';

final logger = Logger(
  printer: PrettyPrinter(
    methodCount: 0,
    colors: true,
    lineLength: 80,
    printEmojis: true,
    dateTimeFormat: DateTimeFormat.dateAndTime,
  ),
);
