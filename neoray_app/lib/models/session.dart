import 'package:freezed_annotation/freezed_annotation.dart';
import 'message.dart';

part 'session.freezed.dart';
part 'session.g.dart';

@freezed
class Session with _$Session {
  const factory Session({
    required String id,
    String? title,
    @Default([]) List<Message> messages,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) = _Session;

  factory Session.create({String? title}) {
    final now = DateTime.now();
    return Session(
      id: now.microsecondsSinceEpoch.toString(),
      title: title ?? '新聊天',
      messages: [],
      createdAt: now,
      updatedAt: now,
    );
  }

  factory Session.fromJson(Map<String, dynamic> json) =>
      _$SessionFromJson(json);
}
