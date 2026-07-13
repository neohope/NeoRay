import 'package:flutter/material.dart';

class AppTheme {
  // 主色调
  static const Color primary = Color(0xFF6366F1);
  static const Color primaryLight = Color(0xFF818CF8);
  static const Color primaryDark = Color(0xFF4F46E5);

  // 功能色
  static const Color secondary = Color(0xFF10B981);
  static const Color accent = Color(0xFFF59E0B);
  static const Color danger = Color(0xFFEF4444);

  // 浅色主题颜色
  static const Color backgroundLight = Color(0xFFF8F9FA);
  static const Color surfaceLight = Color(0xFFFFFFFF);
  static const Color textPrimaryLight = Color(0xFF1F2937);
  static const Color textSecondaryLight = Color(0xFF64748B);

  // 深色主题颜色
  static const Color backgroundDark = Color(0xFF0F172A);
  static const Color surfaceDark = Color(0xFF1E293B);
  static const Color textPrimaryDark = Color(0xFFF1F5F9);
  static const Color textSecondaryDark = Color(0xFF94A3B8);

  // 补充颜色 - 通用界面元素
  static const Color sidebarBackgroundLight = Color(0xFFF0F2F5);
  static const Color sidebarBackgroundDark = Color(0xFF1E293B);

  static const Color borderLight = Color(0xFFE5E7EB);
  static const Color borderDark = Color(0xFF374151);

  static const Color inputBackgroundLight = Color(0xFFF9FAFB);
  static const Color inputBackgroundDark = Color(0xFF1E293B);

  static const Color selectedItemBackgroundLight = Color(0xFFE5E7EB);
  static const Color selectedItemBackgroundDark = Color(0xFF334155);

  // 半透明颜色
  static Color get primaryTransparent10 => primary.withValues(alpha: 0.1);
  static Color get primaryTransparent20 => primary.withValues(alpha: 0.2);
  static Color get dangerTransparent60 => danger.withValues(alpha: 0.6);
  static Color get whiteTransparent10 => Colors.white.withValues(alpha: 0.1);
  static Color get whiteTransparent70 => Colors.white.withValues(alpha: 0.7);

  static final ThemeData lightTheme = ThemeData(
    useMaterial3: true,
    colorScheme: ColorScheme.fromSeed(
      seedColor: primary,
      brightness: Brightness.light,
      primary: primary,
      onPrimary: Colors.white,
      secondary: secondary,
      onSecondary: Colors.white,
      error: danger,
      onError: Colors.white,
      surface: surfaceLight,
      onSurface: textPrimaryLight,
    ),
    scaffoldBackgroundColor: backgroundLight,
    cardColor: surfaceLight,
    textTheme: const TextTheme(
      headlineSmall: TextStyle(
        fontSize: 20,
        fontWeight: FontWeight.bold,
        color: textPrimaryLight,
      ),
      titleMedium: TextStyle(
        fontSize: 16,
        fontWeight: FontWeight.w600,
        color: textPrimaryLight,
      ),
      bodyLarge: TextStyle(
        fontSize: 15,
        color: textPrimaryLight,
        height: 1.6,
      ),
      bodyMedium: TextStyle(
        fontSize: 14,
        color: textSecondaryLight,
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: inputBackgroundLight,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: borderLight),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: borderLight),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: primary),
      ),
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        elevation: 0,
        padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(8),
        ),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(8),
        ),
      ),
    ),
    sliderTheme: const SliderThemeData(),
    switchTheme: const SwitchThemeData(),
  );

  static final ThemeData darkTheme = ThemeData(
    useMaterial3: true,
    colorScheme: ColorScheme.fromSeed(
      seedColor: primary,
      brightness: Brightness.dark,
      primary: primaryLight,
      onPrimary: Colors.white,
      secondary: secondary,
      onSecondary: Colors.white,
      error: danger,
      onError: Colors.white,
      surface: surfaceDark,
      onSurface: textPrimaryDark,
    ),
    scaffoldBackgroundColor: backgroundDark,
    cardColor: surfaceDark,
    textTheme: const TextTheme(
      headlineSmall: TextStyle(
        fontSize: 20,
        fontWeight: FontWeight.bold,
        color: textPrimaryDark,
      ),
      titleMedium: TextStyle(
        fontSize: 16,
        fontWeight: FontWeight.w600,
        color: textPrimaryDark,
      ),
      bodyLarge: TextStyle(
        fontSize: 15,
        color: textPrimaryDark,
        height: 1.6,
      ),
      bodyMedium: TextStyle(
        fontSize: 14,
        color: textSecondaryDark,
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: inputBackgroundDark,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: borderDark),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: borderDark),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: const BorderSide(color: primaryLight),
      ),
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
    ),
    sliderTheme: const SliderThemeData(),
    switchTheme: const SwitchThemeData(),
  );
}
