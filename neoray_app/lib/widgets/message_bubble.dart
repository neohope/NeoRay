import 'package:flutter/material.dart';
import '../models/message.dart';
import '../theme/app_theme.dart';
import '../constants/constants.dart';

class MessageBubble extends StatelessWidget {
  final Message message;
  final bool isStreaming;

  const MessageBubble({
    super.key,
    required this.message,
    this.isStreaming = false,
  });

  bool get isUser => message.role == 'user';

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppDimensions.spacingMd),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment:
            isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        children: [
          if (!isUser) ...[
            _buildAvatar(context),
            const SizedBox(width: AppDimensions.spacingMd),
          ],
          Flexible(
            child: ConstrainedBox(
              constraints: BoxConstraints(
                maxWidth: MediaQuery.of(context).size.width * AppDimensions.messageBubbleMaxWidthRatio,
              ),
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: AppDimensions.messageBubbleHorizontalPadding,
                  vertical: AppDimensions.messageBubbleVerticalPadding,
                ),
                decoration: BoxDecoration(
                  color: isUser
                      ? AppTheme.primary
                      : Theme.of(context).cardColor,
                  borderRadius: BorderRadius.circular(AppDimensions.borderRadiusLg),
                  border: isUser
                      ? null
                      : Border.all(color: AppTheme.borderLight),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (message.reasoningContent != null && message.reasoningContent!.isNotEmpty)
                      _buildReasoningContent(context),
                    _buildContent(context),
                    if (message.toolCalls.isNotEmpty)
                      _buildToolCalls(context),
                    if (isStreaming)
                      Padding(
                        padding: const EdgeInsets.only(top: AppDimensions.spacingSm),
                        child: SizedBox(
                          width: AppDimensions.loadingIndicatorSize,
                          height: AppDimensions.loadingIndicatorSize,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: colorScheme.onSurface.withValues(alpha: 0.5),
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ),
          if (isUser) const SizedBox(width: AppDimensions.spacingMd),
        ],
      ),
    );
  }

  Widget _buildAvatar(BuildContext context) {
    return Container(
      width: AppDimensions.avatarContainerSize,
      height: AppDimensions.avatarContainerSize,
      decoration: BoxDecoration(
        color: AppTheme.primary,
        borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
      ),
      child: const Icon(
        Icons.smart_toy,
        color: Colors.white,
        size: AppDimensions.iconSizeMedium,
      ),
    );
  }

  Widget _buildReasoningContent(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppDimensions.spacingMd),
      child: Container(
        padding: const EdgeInsets.all(AppDimensions.spacingMd),
        decoration: BoxDecoration(
          color: AppTheme.primaryTransparent10,
          borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
          border: Border.all(
            color: AppTheme.primaryTransparent20,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(
                  Icons.psychology,
                  size: 16,
                  color: AppTheme.primary,
                ),
                const SizedBox(width: AppDimensions.spacingSm),
                Text(
                  AppStrings.thinking,
                  style: const TextStyle(
                    fontSize: AppDimensions.fontSizeXs,
                    fontWeight: FontWeight.w600,
                    color: AppTheme.primary,
                  ),
                ),
                if (!message.isReasoningComplete)
                  Padding(
                    padding: const EdgeInsets.only(left: AppDimensions.spacingSm),
                    child: SizedBox(
                      width: AppDimensions.loadingIndicatorSize,
                      height: AppDimensions.loadingIndicatorSize,
                      child: const CircularProgressIndicator(
                        strokeWidth: 2,
                        color: AppTheme.primary,
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: AppDimensions.spacingSm),
            Text(
              message.reasoningContent!,
              style: const TextStyle(
                fontSize: AppDimensions.fontSizeSm,
                height: AppDimensions.lineHeightMd,
                color: AppTheme.textSecondaryLight,
                fontStyle: FontStyle.italic,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context) {
    return Text(
      message.content,
      style: TextStyle(
        fontSize: AppDimensions.fontSizeMd,
        height: AppDimensions.lineHeightLg,
        color: isUser ? Colors.white : AppTheme.textPrimaryLight,
      ),
    );
  }

  Widget _buildToolCalls(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: AppDimensions.spacingMd),
      child: Container(
        padding: const EdgeInsets.all(AppDimensions.spacingMd),
        decoration: BoxDecoration(
          color: isUser
              ? AppTheme.whiteTransparent10
              : AppTheme.backgroundLight,
          borderRadius: BorderRadius.circular(AppDimensions.borderRadiusSm),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: message.toolCalls.map((toolCall) {
            return Padding(
              padding: const EdgeInsets.symmetric(vertical: AppDimensions.spacingXs),
              child: Row(
                children: [
                  Icon(
                    Icons.build_circle_outlined,
                    size: 16,
                    color: isUser ? AppTheme.whiteTransparent70 : AppTheme.textSecondaryLight,
                  ),
                  const SizedBox(width: AppDimensions.spacingSm),
                  Expanded(
                    child: Text(
                      '${toolCall.name}(${toolCall.arguments})',
                      style: TextStyle(
                        fontSize: AppDimensions.fontSizeXs,
                        color: isUser ? AppTheme.whiteTransparent70 : AppTheme.textSecondaryLight,
                      ),
                    ),
                  ),
                ],
              ),
            );
          }).toList(),
        ),
      ),
    );
  }
}
