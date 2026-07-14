// coverage:ignore-file
// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'app_config.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

T _$identity<T>(T value) => value;

final _privateConstructorUsedError = UnsupportedError(
    'It seems like you constructed your class using `MyClass._()`. This constructor is only meant to be used by freezed and you are not supposed to need it nor use it.\nPlease check the documentation here for more information: https://github.com/rrousselGit/freezed#adding-getters-and-methods-to-our-models');

AppConfig _$AppConfigFromJson(Map<String, dynamic> json) {
  return _AppConfig.fromJson(json);
}

/// @nodoc
mixin _$AppConfig {
  String get serverUrl => throw _privateConstructorUsedError;
  String get themeMode => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $AppConfigCopyWith<AppConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $AppConfigCopyWith<$Res> {
  factory $AppConfigCopyWith(AppConfig value, $Res Function(AppConfig) then) =
      _$AppConfigCopyWithImpl<$Res, AppConfig>;
  @useResult
  $Res call({String serverUrl, String themeMode});
}

/// @nodoc
class _$AppConfigCopyWithImpl<$Res, $Val extends AppConfig>
    implements $AppConfigCopyWith<$Res> {
  _$AppConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? serverUrl = null,
    Object? themeMode = null,
  }) {
    return _then(_value.copyWith(
      serverUrl: null == serverUrl
          ? _value.serverUrl
          : serverUrl // ignore: cast_nullable_to_non_nullable
              as String,
      themeMode: null == themeMode
          ? _value.themeMode
          : themeMode // ignore: cast_nullable_to_non_nullable
              as String,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$AppConfigImplCopyWith<$Res>
    implements $AppConfigCopyWith<$Res> {
  factory _$$AppConfigImplCopyWith(
          _$AppConfigImpl value, $Res Function(_$AppConfigImpl) then) =
      __$$AppConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call({String serverUrl, String themeMode});
}

/// @nodoc
class __$$AppConfigImplCopyWithImpl<$Res>
    extends _$AppConfigCopyWithImpl<$Res, _$AppConfigImpl>
    implements _$$AppConfigImplCopyWith<$Res> {
  __$$AppConfigImplCopyWithImpl(
      _$AppConfigImpl _value, $Res Function(_$AppConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? serverUrl = null,
    Object? themeMode = null,
  }) {
    return _then(_$AppConfigImpl(
      serverUrl: null == serverUrl
          ? _value.serverUrl
          : serverUrl // ignore: cast_nullable_to_non_nullable
              as String,
      themeMode: null == themeMode
          ? _value.themeMode
          : themeMode // ignore: cast_nullable_to_non_nullable
              as String,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$AppConfigImpl implements _AppConfig {
  const _$AppConfigImpl(
      {this.serverUrl = AppDefaults.defaultServerUrl,
      this.themeMode = AppDefaults.defaultThemeMode});

  factory _$AppConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$AppConfigImplFromJson(json);

  @override
  @JsonKey()
  final String serverUrl;
  @override
  @JsonKey()
  final String themeMode;

  @override
  String toString() {
    return 'AppConfig(serverUrl: $serverUrl, themeMode: $themeMode)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$AppConfigImpl &&
            (identical(other.serverUrl, serverUrl) ||
                other.serverUrl == serverUrl) &&
            (identical(other.themeMode, themeMode) ||
                other.themeMode == themeMode));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, serverUrl, themeMode);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$AppConfigImplCopyWith<_$AppConfigImpl> get copyWith =>
      __$$AppConfigImplCopyWithImpl<_$AppConfigImpl>(this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$AppConfigImplToJson(
      this,
    );
  }
}

abstract class _AppConfig implements AppConfig {
  const factory _AppConfig({final String serverUrl, final String themeMode}) =
      _$AppConfigImpl;

  factory _AppConfig.fromJson(Map<String, dynamic> json) =
      _$AppConfigImpl.fromJson;

  @override
  String get serverUrl;
  @override
  String get themeMode;
  @override
  @JsonKey(ignore: true)
  _$$AppConfigImplCopyWith<_$AppConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

ServerConfig _$ServerConfigFromJson(Map<String, dynamic> json) {
  return _ServerConfig.fromJson(json);
}

/// @nodoc
mixin _$ServerConfig {
  ServerLLMConfig get llm => throw _privateConstructorUsedError;
  ServerChannelConfig get channels => throw _privateConstructorUsedError;
  ServerToolConfig get tools => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ServerConfigCopyWith<ServerConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ServerConfigCopyWith<$Res> {
  factory $ServerConfigCopyWith(
          ServerConfig value, $Res Function(ServerConfig) then) =
      _$ServerConfigCopyWithImpl<$Res, ServerConfig>;
  @useResult
  $Res call(
      {ServerLLMConfig llm,
      ServerChannelConfig channels,
      ServerToolConfig tools});

  $ServerLLMConfigCopyWith<$Res> get llm;
  $ServerChannelConfigCopyWith<$Res> get channels;
  $ServerToolConfigCopyWith<$Res> get tools;
}

/// @nodoc
class _$ServerConfigCopyWithImpl<$Res, $Val extends ServerConfig>
    implements $ServerConfigCopyWith<$Res> {
  _$ServerConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? llm = null,
    Object? channels = null,
    Object? tools = null,
  }) {
    return _then(_value.copyWith(
      llm: null == llm
          ? _value.llm
          : llm // ignore: cast_nullable_to_non_nullable
              as ServerLLMConfig,
      channels: null == channels
          ? _value.channels
          : channels // ignore: cast_nullable_to_non_nullable
              as ServerChannelConfig,
      tools: null == tools
          ? _value.tools
          : tools // ignore: cast_nullable_to_non_nullable
              as ServerToolConfig,
    ) as $Val);
  }

  @override
  @pragma('vm:prefer-inline')
  $ServerLLMConfigCopyWith<$Res> get llm {
    return $ServerLLMConfigCopyWith<$Res>(_value.llm, (value) {
      return _then(_value.copyWith(llm: value) as $Val);
    });
  }

  @override
  @pragma('vm:prefer-inline')
  $ServerChannelConfigCopyWith<$Res> get channels {
    return $ServerChannelConfigCopyWith<$Res>(_value.channels, (value) {
      return _then(_value.copyWith(channels: value) as $Val);
    });
  }

  @override
  @pragma('vm:prefer-inline')
  $ServerToolConfigCopyWith<$Res> get tools {
    return $ServerToolConfigCopyWith<$Res>(_value.tools, (value) {
      return _then(_value.copyWith(tools: value) as $Val);
    });
  }
}

/// @nodoc
abstract class _$$ServerConfigImplCopyWith<$Res>
    implements $ServerConfigCopyWith<$Res> {
  factory _$$ServerConfigImplCopyWith(
          _$ServerConfigImpl value, $Res Function(_$ServerConfigImpl) then) =
      __$$ServerConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call(
      {ServerLLMConfig llm,
      ServerChannelConfig channels,
      ServerToolConfig tools});

  @override
  $ServerLLMConfigCopyWith<$Res> get llm;
  @override
  $ServerChannelConfigCopyWith<$Res> get channels;
  @override
  $ServerToolConfigCopyWith<$Res> get tools;
}

/// @nodoc
class __$$ServerConfigImplCopyWithImpl<$Res>
    extends _$ServerConfigCopyWithImpl<$Res, _$ServerConfigImpl>
    implements _$$ServerConfigImplCopyWith<$Res> {
  __$$ServerConfigImplCopyWithImpl(
      _$ServerConfigImpl _value, $Res Function(_$ServerConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? llm = null,
    Object? channels = null,
    Object? tools = null,
  }) {
    return _then(_$ServerConfigImpl(
      llm: null == llm
          ? _value.llm
          : llm // ignore: cast_nullable_to_non_nullable
              as ServerLLMConfig,
      channels: null == channels
          ? _value.channels
          : channels // ignore: cast_nullable_to_non_nullable
              as ServerChannelConfig,
      tools: null == tools
          ? _value.tools
          : tools // ignore: cast_nullable_to_non_nullable
              as ServerToolConfig,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$ServerConfigImpl implements _ServerConfig {
  const _$ServerConfigImpl(
      {this.llm = const ServerLLMConfig(),
      this.channels = const ServerChannelConfig(),
      this.tools = const ServerToolConfig()});

  factory _$ServerConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ServerConfigImplFromJson(json);

  @override
  @JsonKey()
  final ServerLLMConfig llm;
  @override
  @JsonKey()
  final ServerChannelConfig channels;
  @override
  @JsonKey()
  final ServerToolConfig tools;

  @override
  String toString() {
    return 'ServerConfig(llm: $llm, channels: $channels, tools: $tools)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ServerConfigImpl &&
            (identical(other.llm, llm) || other.llm == llm) &&
            (identical(other.channels, channels) ||
                other.channels == channels) &&
            (identical(other.tools, tools) || other.tools == tools));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, llm, channels, tools);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ServerConfigImplCopyWith<_$ServerConfigImpl> get copyWith =>
      __$$ServerConfigImplCopyWithImpl<_$ServerConfigImpl>(this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ServerConfigImplToJson(
      this,
    );
  }
}

abstract class _ServerConfig implements ServerConfig {
  const factory _ServerConfig(
      {final ServerLLMConfig llm,
      final ServerChannelConfig channels,
      final ServerToolConfig tools}) = _$ServerConfigImpl;

  factory _ServerConfig.fromJson(Map<String, dynamic> json) =
      _$ServerConfigImpl.fromJson;

  @override
  ServerLLMConfig get llm;
  @override
  ServerChannelConfig get channels;
  @override
  ServerToolConfig get tools;
  @override
  @JsonKey(ignore: true)
  _$$ServerConfigImplCopyWith<_$ServerConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

ServerLLMConfig _$ServerLLMConfigFromJson(Map<String, dynamic> json) {
  return _ServerLLMConfig.fromJson(json);
}

/// @nodoc
mixin _$ServerLLMConfig {
  String get defaultProvider => throw _privateConstructorUsedError;
  Map<String, ProviderConfig> get providers =>
      throw _privateConstructorUsedError;
  List<FallbackModelConfig> get fallbackModels =>
      throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ServerLLMConfigCopyWith<ServerLLMConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ServerLLMConfigCopyWith<$Res> {
  factory $ServerLLMConfigCopyWith(
          ServerLLMConfig value, $Res Function(ServerLLMConfig) then) =
      _$ServerLLMConfigCopyWithImpl<$Res, ServerLLMConfig>;
  @useResult
  $Res call(
      {String defaultProvider,
      Map<String, ProviderConfig> providers,
      List<FallbackModelConfig> fallbackModels});
}

/// @nodoc
class _$ServerLLMConfigCopyWithImpl<$Res, $Val extends ServerLLMConfig>
    implements $ServerLLMConfigCopyWith<$Res> {
  _$ServerLLMConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? defaultProvider = null,
    Object? providers = null,
    Object? fallbackModels = null,
  }) {
    return _then(_value.copyWith(
      defaultProvider: null == defaultProvider
          ? _value.defaultProvider
          : defaultProvider // ignore: cast_nullable_to_non_nullable
              as String,
      providers: null == providers
          ? _value.providers
          : providers // ignore: cast_nullable_to_non_nullable
              as Map<String, ProviderConfig>,
      fallbackModels: null == fallbackModels
          ? _value.fallbackModels
          : fallbackModels // ignore: cast_nullable_to_non_nullable
              as List<FallbackModelConfig>,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$ServerLLMConfigImplCopyWith<$Res>
    implements $ServerLLMConfigCopyWith<$Res> {
  factory _$$ServerLLMConfigImplCopyWith(_$ServerLLMConfigImpl value,
          $Res Function(_$ServerLLMConfigImpl) then) =
      __$$ServerLLMConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call(
      {String defaultProvider,
      Map<String, ProviderConfig> providers,
      List<FallbackModelConfig> fallbackModels});
}

/// @nodoc
class __$$ServerLLMConfigImplCopyWithImpl<$Res>
    extends _$ServerLLMConfigCopyWithImpl<$Res, _$ServerLLMConfigImpl>
    implements _$$ServerLLMConfigImplCopyWith<$Res> {
  __$$ServerLLMConfigImplCopyWithImpl(
      _$ServerLLMConfigImpl _value, $Res Function(_$ServerLLMConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? defaultProvider = null,
    Object? providers = null,
    Object? fallbackModels = null,
  }) {
    return _then(_$ServerLLMConfigImpl(
      defaultProvider: null == defaultProvider
          ? _value.defaultProvider
          : defaultProvider // ignore: cast_nullable_to_non_nullable
              as String,
      providers: null == providers
          ? _value._providers
          : providers // ignore: cast_nullable_to_non_nullable
              as Map<String, ProviderConfig>,
      fallbackModels: null == fallbackModels
          ? _value._fallbackModels
          : fallbackModels // ignore: cast_nullable_to_non_nullable
              as List<FallbackModelConfig>,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$ServerLLMConfigImpl implements _ServerLLMConfig {
  const _$ServerLLMConfigImpl(
      {this.defaultProvider = AppDefaults.defaultLLMProvider,
      final Map<String, ProviderConfig> providers = const {},
      final List<FallbackModelConfig> fallbackModels = const []})
      : _providers = providers,
        _fallbackModels = fallbackModels;

  factory _$ServerLLMConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ServerLLMConfigImplFromJson(json);

  @override
  @JsonKey()
  final String defaultProvider;
  final Map<String, ProviderConfig> _providers;
  @override
  @JsonKey()
  Map<String, ProviderConfig> get providers {
    if (_providers is EqualUnmodifiableMapView) return _providers;
    // ignore: implicit_dynamic_type
    return EqualUnmodifiableMapView(_providers);
  }

  final List<FallbackModelConfig> _fallbackModels;
  @override
  @JsonKey()
  List<FallbackModelConfig> get fallbackModels {
    if (_fallbackModels is EqualUnmodifiableListView) return _fallbackModels;
    // ignore: implicit_dynamic_type
    return EqualUnmodifiableListView(_fallbackModels);
  }

  @override
  String toString() {
    return 'ServerLLMConfig(defaultProvider: $defaultProvider, providers: $providers, fallbackModels: $fallbackModels)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ServerLLMConfigImpl &&
            (identical(other.defaultProvider, defaultProvider) ||
                other.defaultProvider == defaultProvider) &&
            const DeepCollectionEquality()
                .equals(other._providers, _providers) &&
            const DeepCollectionEquality()
                .equals(other._fallbackModels, _fallbackModels));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(
      runtimeType,
      defaultProvider,
      const DeepCollectionEquality().hash(_providers),
      const DeepCollectionEquality().hash(_fallbackModels));

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ServerLLMConfigImplCopyWith<_$ServerLLMConfigImpl> get copyWith =>
      __$$ServerLLMConfigImplCopyWithImpl<_$ServerLLMConfigImpl>(
          this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ServerLLMConfigImplToJson(
      this,
    );
  }
}

abstract class _ServerLLMConfig implements ServerLLMConfig {
  const factory _ServerLLMConfig(
      {final String defaultProvider,
      final Map<String, ProviderConfig> providers,
      final List<FallbackModelConfig> fallbackModels}) = _$ServerLLMConfigImpl;

  factory _ServerLLMConfig.fromJson(Map<String, dynamic> json) =
      _$ServerLLMConfigImpl.fromJson;

  @override
  String get defaultProvider;
  @override
  Map<String, ProviderConfig> get providers;
  @override
  List<FallbackModelConfig> get fallbackModels;
  @override
  @JsonKey(ignore: true)
  _$$ServerLLMConfigImplCopyWith<_$ServerLLMConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

ProviderConfig _$ProviderConfigFromJson(Map<String, dynamic> json) {
  return _ProviderConfig.fromJson(json);
}

/// @nodoc
mixin _$ProviderConfig {
  String get apiKey => throw _privateConstructorUsedError;
  String get apiUrl => throw _privateConstructorUsedError;
  String get model => throw _privateConstructorUsedError;
  int get maxTokens => throw _privateConstructorUsedError;
  double get temperature => throw _privateConstructorUsedError;
  double get timeout => throw _privateConstructorUsedError;
  String get reasoningEffort => throw _privateConstructorUsedError;
  bool get promptCacheEnabled => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ProviderConfigCopyWith<ProviderConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ProviderConfigCopyWith<$Res> {
  factory $ProviderConfigCopyWith(
          ProviderConfig value, $Res Function(ProviderConfig) then) =
      _$ProviderConfigCopyWithImpl<$Res, ProviderConfig>;
  @useResult
  $Res call(
      {String apiKey,
      String apiUrl,
      String model,
      int maxTokens,
      double temperature,
      double timeout,
      String reasoningEffort,
      bool promptCacheEnabled});
}

/// @nodoc
class _$ProviderConfigCopyWithImpl<$Res, $Val extends ProviderConfig>
    implements $ProviderConfigCopyWith<$Res> {
  _$ProviderConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? apiKey = null,
    Object? apiUrl = null,
    Object? model = null,
    Object? maxTokens = null,
    Object? temperature = null,
    Object? timeout = null,
    Object? reasoningEffort = null,
    Object? promptCacheEnabled = null,
  }) {
    return _then(_value.copyWith(
      apiKey: null == apiKey
          ? _value.apiKey
          : apiKey // ignore: cast_nullable_to_non_nullable
              as String,
      apiUrl: null == apiUrl
          ? _value.apiUrl
          : apiUrl // ignore: cast_nullable_to_non_nullable
              as String,
      model: null == model
          ? _value.model
          : model // ignore: cast_nullable_to_non_nullable
              as String,
      maxTokens: null == maxTokens
          ? _value.maxTokens
          : maxTokens // ignore: cast_nullable_to_non_nullable
              as int,
      temperature: null == temperature
          ? _value.temperature
          : temperature // ignore: cast_nullable_to_non_nullable
              as double,
      timeout: null == timeout
          ? _value.timeout
          : timeout // ignore: cast_nullable_to_non_nullable
              as double,
      reasoningEffort: null == reasoningEffort
          ? _value.reasoningEffort
          : reasoningEffort // ignore: cast_nullable_to_non_nullable
              as String,
      promptCacheEnabled: null == promptCacheEnabled
          ? _value.promptCacheEnabled
          : promptCacheEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$ProviderConfigImplCopyWith<$Res>
    implements $ProviderConfigCopyWith<$Res> {
  factory _$$ProviderConfigImplCopyWith(_$ProviderConfigImpl value,
          $Res Function(_$ProviderConfigImpl) then) =
      __$$ProviderConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call(
      {String apiKey,
      String apiUrl,
      String model,
      int maxTokens,
      double temperature,
      double timeout,
      String reasoningEffort,
      bool promptCacheEnabled});
}

/// @nodoc
class __$$ProviderConfigImplCopyWithImpl<$Res>
    extends _$ProviderConfigCopyWithImpl<$Res, _$ProviderConfigImpl>
    implements _$$ProviderConfigImplCopyWith<$Res> {
  __$$ProviderConfigImplCopyWithImpl(
      _$ProviderConfigImpl _value, $Res Function(_$ProviderConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? apiKey = null,
    Object? apiUrl = null,
    Object? model = null,
    Object? maxTokens = null,
    Object? temperature = null,
    Object? timeout = null,
    Object? reasoningEffort = null,
    Object? promptCacheEnabled = null,
  }) {
    return _then(_$ProviderConfigImpl(
      apiKey: null == apiKey
          ? _value.apiKey
          : apiKey // ignore: cast_nullable_to_non_nullable
              as String,
      apiUrl: null == apiUrl
          ? _value.apiUrl
          : apiUrl // ignore: cast_nullable_to_non_nullable
              as String,
      model: null == model
          ? _value.model
          : model // ignore: cast_nullable_to_non_nullable
              as String,
      maxTokens: null == maxTokens
          ? _value.maxTokens
          : maxTokens // ignore: cast_nullable_to_non_nullable
              as int,
      temperature: null == temperature
          ? _value.temperature
          : temperature // ignore: cast_nullable_to_non_nullable
              as double,
      timeout: null == timeout
          ? _value.timeout
          : timeout // ignore: cast_nullable_to_non_nullable
              as double,
      reasoningEffort: null == reasoningEffort
          ? _value.reasoningEffort
          : reasoningEffort // ignore: cast_nullable_to_non_nullable
              as String,
      promptCacheEnabled: null == promptCacheEnabled
          ? _value.promptCacheEnabled
          : promptCacheEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$ProviderConfigImpl implements _ProviderConfig {
  const _$ProviderConfigImpl(
      {this.apiKey = '',
      this.apiUrl = '',
      this.model = AppDefaults.defaultLLMModel,
      this.maxTokens = AppDefaults.defaultMaxTokens,
      this.temperature = AppDefaults.defaultTemperature,
      this.timeout = AppDefaults.defaultTimeoutSec,
      this.reasoningEffort = AppDefaults.defaultReasoningEffort,
      this.promptCacheEnabled = AppDefaults.defaultPromptCacheEnabled});

  factory _$ProviderConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ProviderConfigImplFromJson(json);

  @override
  @JsonKey()
  final String apiKey;
  @override
  @JsonKey()
  final String apiUrl;
  @override
  @JsonKey()
  final String model;
  @override
  @JsonKey()
  final int maxTokens;
  @override
  @JsonKey()
  final double temperature;
  @override
  @JsonKey()
  final double timeout;
  @override
  @JsonKey()
  final String reasoningEffort;
  @override
  @JsonKey()
  final bool promptCacheEnabled;

  @override
  String toString() {
    return 'ProviderConfig(apiKey: $apiKey, apiUrl: $apiUrl, model: $model, maxTokens: $maxTokens, temperature: $temperature, timeout: $timeout, reasoningEffort: $reasoningEffort, promptCacheEnabled: $promptCacheEnabled)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ProviderConfigImpl &&
            (identical(other.apiKey, apiKey) || other.apiKey == apiKey) &&
            (identical(other.apiUrl, apiUrl) || other.apiUrl == apiUrl) &&
            (identical(other.model, model) || other.model == model) &&
            (identical(other.maxTokens, maxTokens) ||
                other.maxTokens == maxTokens) &&
            (identical(other.temperature, temperature) ||
                other.temperature == temperature) &&
            (identical(other.timeout, timeout) || other.timeout == timeout) &&
            (identical(other.reasoningEffort, reasoningEffort) ||
                other.reasoningEffort == reasoningEffort) &&
            (identical(other.promptCacheEnabled, promptCacheEnabled) ||
                other.promptCacheEnabled == promptCacheEnabled));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, apiKey, apiUrl, model, maxTokens,
      temperature, timeout, reasoningEffort, promptCacheEnabled);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ProviderConfigImplCopyWith<_$ProviderConfigImpl> get copyWith =>
      __$$ProviderConfigImplCopyWithImpl<_$ProviderConfigImpl>(
          this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ProviderConfigImplToJson(
      this,
    );
  }
}

abstract class _ProviderConfig implements ProviderConfig {
  const factory _ProviderConfig(
      {final String apiKey,
      final String apiUrl,
      final String model,
      final int maxTokens,
      final double temperature,
      final double timeout,
      final String reasoningEffort,
      final bool promptCacheEnabled}) = _$ProviderConfigImpl;

  factory _ProviderConfig.fromJson(Map<String, dynamic> json) =
      _$ProviderConfigImpl.fromJson;

  @override
  String get apiKey;
  @override
  String get apiUrl;
  @override
  String get model;
  @override
  int get maxTokens;
  @override
  double get temperature;
  @override
  double get timeout;
  @override
  String get reasoningEffort;
  @override
  bool get promptCacheEnabled;
  @override
  @JsonKey(ignore: true)
  _$$ProviderConfigImplCopyWith<_$ProviderConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

FallbackModelConfig _$FallbackModelConfigFromJson(Map<String, dynamic> json) {
  return _FallbackModelConfig.fromJson(json);
}

/// @nodoc
mixin _$FallbackModelConfig {
  String? get model => throw _privateConstructorUsedError;
  String? get provider => throw _privateConstructorUsedError;
  int? get maxTokens => throw _privateConstructorUsedError;
  double? get temperature => throw _privateConstructorUsedError;
  String? get reasoningEffort => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $FallbackModelConfigCopyWith<FallbackModelConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $FallbackModelConfigCopyWith<$Res> {
  factory $FallbackModelConfigCopyWith(
          FallbackModelConfig value, $Res Function(FallbackModelConfig) then) =
      _$FallbackModelConfigCopyWithImpl<$Res, FallbackModelConfig>;
  @useResult
  $Res call(
      {String? model,
      String? provider,
      int? maxTokens,
      double? temperature,
      String? reasoningEffort});
}

/// @nodoc
class _$FallbackModelConfigCopyWithImpl<$Res, $Val extends FallbackModelConfig>
    implements $FallbackModelConfigCopyWith<$Res> {
  _$FallbackModelConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? model = freezed,
    Object? provider = freezed,
    Object? maxTokens = freezed,
    Object? temperature = freezed,
    Object? reasoningEffort = freezed,
  }) {
    return _then(_value.copyWith(
      model: freezed == model
          ? _value.model
          : model // ignore: cast_nullable_to_non_nullable
              as String?,
      provider: freezed == provider
          ? _value.provider
          : provider // ignore: cast_nullable_to_non_nullable
              as String?,
      maxTokens: freezed == maxTokens
          ? _value.maxTokens
          : maxTokens // ignore: cast_nullable_to_non_nullable
              as int?,
      temperature: freezed == temperature
          ? _value.temperature
          : temperature // ignore: cast_nullable_to_non_nullable
              as double?,
      reasoningEffort: freezed == reasoningEffort
          ? _value.reasoningEffort
          : reasoningEffort // ignore: cast_nullable_to_non_nullable
              as String?,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$FallbackModelConfigImplCopyWith<$Res>
    implements $FallbackModelConfigCopyWith<$Res> {
  factory _$$FallbackModelConfigImplCopyWith(_$FallbackModelConfigImpl value,
          $Res Function(_$FallbackModelConfigImpl) then) =
      __$$FallbackModelConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call(
      {String? model,
      String? provider,
      int? maxTokens,
      double? temperature,
      String? reasoningEffort});
}

/// @nodoc
class __$$FallbackModelConfigImplCopyWithImpl<$Res>
    extends _$FallbackModelConfigCopyWithImpl<$Res, _$FallbackModelConfigImpl>
    implements _$$FallbackModelConfigImplCopyWith<$Res> {
  __$$FallbackModelConfigImplCopyWithImpl(_$FallbackModelConfigImpl _value,
      $Res Function(_$FallbackModelConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? model = freezed,
    Object? provider = freezed,
    Object? maxTokens = freezed,
    Object? temperature = freezed,
    Object? reasoningEffort = freezed,
  }) {
    return _then(_$FallbackModelConfigImpl(
      model: freezed == model
          ? _value.model
          : model // ignore: cast_nullable_to_non_nullable
              as String?,
      provider: freezed == provider
          ? _value.provider
          : provider // ignore: cast_nullable_to_non_nullable
              as String?,
      maxTokens: freezed == maxTokens
          ? _value.maxTokens
          : maxTokens // ignore: cast_nullable_to_non_nullable
              as int?,
      temperature: freezed == temperature
          ? _value.temperature
          : temperature // ignore: cast_nullable_to_non_nullable
              as double?,
      reasoningEffort: freezed == reasoningEffort
          ? _value.reasoningEffort
          : reasoningEffort // ignore: cast_nullable_to_non_nullable
              as String?,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$FallbackModelConfigImpl implements _FallbackModelConfig {
  const _$FallbackModelConfigImpl(
      {this.model,
      this.provider,
      this.maxTokens,
      this.temperature,
      this.reasoningEffort});

  factory _$FallbackModelConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$FallbackModelConfigImplFromJson(json);

  @override
  final String? model;
  @override
  final String? provider;
  @override
  final int? maxTokens;
  @override
  final double? temperature;
  @override
  final String? reasoningEffort;

  @override
  String toString() {
    return 'FallbackModelConfig(model: $model, provider: $provider, maxTokens: $maxTokens, temperature: $temperature, reasoningEffort: $reasoningEffort)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$FallbackModelConfigImpl &&
            (identical(other.model, model) || other.model == model) &&
            (identical(other.provider, provider) ||
                other.provider == provider) &&
            (identical(other.maxTokens, maxTokens) ||
                other.maxTokens == maxTokens) &&
            (identical(other.temperature, temperature) ||
                other.temperature == temperature) &&
            (identical(other.reasoningEffort, reasoningEffort) ||
                other.reasoningEffort == reasoningEffort));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(
      runtimeType, model, provider, maxTokens, temperature, reasoningEffort);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$FallbackModelConfigImplCopyWith<_$FallbackModelConfigImpl> get copyWith =>
      __$$FallbackModelConfigImplCopyWithImpl<_$FallbackModelConfigImpl>(
          this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$FallbackModelConfigImplToJson(
      this,
    );
  }
}

abstract class _FallbackModelConfig implements FallbackModelConfig {
  const factory _FallbackModelConfig(
      {final String? model,
      final String? provider,
      final int? maxTokens,
      final double? temperature,
      final String? reasoningEffort}) = _$FallbackModelConfigImpl;

  factory _FallbackModelConfig.fromJson(Map<String, dynamic> json) =
      _$FallbackModelConfigImpl.fromJson;

  @override
  String? get model;
  @override
  String? get provider;
  @override
  int? get maxTokens;
  @override
  double? get temperature;
  @override
  String? get reasoningEffort;
  @override
  @JsonKey(ignore: true)
  _$$FallbackModelConfigImplCopyWith<_$FallbackModelConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

ServerChannelConfig _$ServerChannelConfigFromJson(Map<String, dynamic> json) {
  return _ServerChannelConfig.fromJson(json);
}

/// @nodoc
mixin _$ServerChannelConfig {
  FeishuConfig get feishu => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ServerChannelConfigCopyWith<ServerChannelConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ServerChannelConfigCopyWith<$Res> {
  factory $ServerChannelConfigCopyWith(
          ServerChannelConfig value, $Res Function(ServerChannelConfig) then) =
      _$ServerChannelConfigCopyWithImpl<$Res, ServerChannelConfig>;
  @useResult
  $Res call({FeishuConfig feishu});

  $FeishuConfigCopyWith<$Res> get feishu;
}

/// @nodoc
class _$ServerChannelConfigCopyWithImpl<$Res, $Val extends ServerChannelConfig>
    implements $ServerChannelConfigCopyWith<$Res> {
  _$ServerChannelConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? feishu = null,
  }) {
    return _then(_value.copyWith(
      feishu: null == feishu
          ? _value.feishu
          : feishu // ignore: cast_nullable_to_non_nullable
              as FeishuConfig,
    ) as $Val);
  }

  @override
  @pragma('vm:prefer-inline')
  $FeishuConfigCopyWith<$Res> get feishu {
    return $FeishuConfigCopyWith<$Res>(_value.feishu, (value) {
      return _then(_value.copyWith(feishu: value) as $Val);
    });
  }
}

/// @nodoc
abstract class _$$ServerChannelConfigImplCopyWith<$Res>
    implements $ServerChannelConfigCopyWith<$Res> {
  factory _$$ServerChannelConfigImplCopyWith(_$ServerChannelConfigImpl value,
          $Res Function(_$ServerChannelConfigImpl) then) =
      __$$ServerChannelConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call({FeishuConfig feishu});

  @override
  $FeishuConfigCopyWith<$Res> get feishu;
}

/// @nodoc
class __$$ServerChannelConfigImplCopyWithImpl<$Res>
    extends _$ServerChannelConfigCopyWithImpl<$Res, _$ServerChannelConfigImpl>
    implements _$$ServerChannelConfigImplCopyWith<$Res> {
  __$$ServerChannelConfigImplCopyWithImpl(_$ServerChannelConfigImpl _value,
      $Res Function(_$ServerChannelConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? feishu = null,
  }) {
    return _then(_$ServerChannelConfigImpl(
      feishu: null == feishu
          ? _value.feishu
          : feishu // ignore: cast_nullable_to_non_nullable
              as FeishuConfig,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$ServerChannelConfigImpl implements _ServerChannelConfig {
  const _$ServerChannelConfigImpl({this.feishu = const FeishuConfig()});

  factory _$ServerChannelConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ServerChannelConfigImplFromJson(json);

  @override
  @JsonKey()
  final FeishuConfig feishu;

  @override
  String toString() {
    return 'ServerChannelConfig(feishu: $feishu)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ServerChannelConfigImpl &&
            (identical(other.feishu, feishu) || other.feishu == feishu));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, feishu);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ServerChannelConfigImplCopyWith<_$ServerChannelConfigImpl> get copyWith =>
      __$$ServerChannelConfigImplCopyWithImpl<_$ServerChannelConfigImpl>(
          this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ServerChannelConfigImplToJson(
      this,
    );
  }
}

abstract class _ServerChannelConfig implements ServerChannelConfig {
  const factory _ServerChannelConfig({final FeishuConfig feishu}) =
      _$ServerChannelConfigImpl;

  factory _ServerChannelConfig.fromJson(Map<String, dynamic> json) =
      _$ServerChannelConfigImpl.fromJson;

  @override
  FeishuConfig get feishu;
  @override
  @JsonKey(ignore: true)
  _$$ServerChannelConfigImplCopyWith<_$ServerChannelConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

FeishuConfig _$FeishuConfigFromJson(Map<String, dynamic> json) {
  return _FeishuConfig.fromJson(json);
}

/// @nodoc
mixin _$FeishuConfig {
  bool get enabled => throw _privateConstructorUsedError;
  String get appId => throw _privateConstructorUsedError;
  String get appSecret => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $FeishuConfigCopyWith<FeishuConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $FeishuConfigCopyWith<$Res> {
  factory $FeishuConfigCopyWith(
          FeishuConfig value, $Res Function(FeishuConfig) then) =
      _$FeishuConfigCopyWithImpl<$Res, FeishuConfig>;
  @useResult
  $Res call({bool enabled, String appId, String appSecret});
}

/// @nodoc
class _$FeishuConfigCopyWithImpl<$Res, $Val extends FeishuConfig>
    implements $FeishuConfigCopyWith<$Res> {
  _$FeishuConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? enabled = null,
    Object? appId = null,
    Object? appSecret = null,
  }) {
    return _then(_value.copyWith(
      enabled: null == enabled
          ? _value.enabled
          : enabled // ignore: cast_nullable_to_non_nullable
              as bool,
      appId: null == appId
          ? _value.appId
          : appId // ignore: cast_nullable_to_non_nullable
              as String,
      appSecret: null == appSecret
          ? _value.appSecret
          : appSecret // ignore: cast_nullable_to_non_nullable
              as String,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$FeishuConfigImplCopyWith<$Res>
    implements $FeishuConfigCopyWith<$Res> {
  factory _$$FeishuConfigImplCopyWith(
          _$FeishuConfigImpl value, $Res Function(_$FeishuConfigImpl) then) =
      __$$FeishuConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call({bool enabled, String appId, String appSecret});
}

/// @nodoc
class __$$FeishuConfigImplCopyWithImpl<$Res>
    extends _$FeishuConfigCopyWithImpl<$Res, _$FeishuConfigImpl>
    implements _$$FeishuConfigImplCopyWith<$Res> {
  __$$FeishuConfigImplCopyWithImpl(
      _$FeishuConfigImpl _value, $Res Function(_$FeishuConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? enabled = null,
    Object? appId = null,
    Object? appSecret = null,
  }) {
    return _then(_$FeishuConfigImpl(
      enabled: null == enabled
          ? _value.enabled
          : enabled // ignore: cast_nullable_to_non_nullable
              as bool,
      appId: null == appId
          ? _value.appId
          : appId // ignore: cast_nullable_to_non_nullable
              as String,
      appSecret: null == appSecret
          ? _value.appSecret
          : appSecret // ignore: cast_nullable_to_non_nullable
              as String,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$FeishuConfigImpl implements _FeishuConfig {
  const _$FeishuConfigImpl(
      {this.enabled = false, this.appId = '', this.appSecret = ''});

  factory _$FeishuConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$FeishuConfigImplFromJson(json);

  @override
  @JsonKey()
  final bool enabled;
  @override
  @JsonKey()
  final String appId;
  @override
  @JsonKey()
  final String appSecret;

  @override
  String toString() {
    return 'FeishuConfig(enabled: $enabled, appId: $appId, appSecret: $appSecret)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$FeishuConfigImpl &&
            (identical(other.enabled, enabled) || other.enabled == enabled) &&
            (identical(other.appId, appId) || other.appId == appId) &&
            (identical(other.appSecret, appSecret) ||
                other.appSecret == appSecret));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, enabled, appId, appSecret);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$FeishuConfigImplCopyWith<_$FeishuConfigImpl> get copyWith =>
      __$$FeishuConfigImplCopyWithImpl<_$FeishuConfigImpl>(this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$FeishuConfigImplToJson(
      this,
    );
  }
}

abstract class _FeishuConfig implements FeishuConfig {
  const factory _FeishuConfig(
      {final bool enabled,
      final String appId,
      final String appSecret}) = _$FeishuConfigImpl;

  factory _FeishuConfig.fromJson(Map<String, dynamic> json) =
      _$FeishuConfigImpl.fromJson;

  @override
  bool get enabled;
  @override
  String get appId;
  @override
  String get appSecret;
  @override
  @JsonKey(ignore: true)
  _$$FeishuConfigImplCopyWith<_$FeishuConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

ServerToolConfig _$ServerToolConfigFromJson(Map<String, dynamic> json) {
  return _ServerToolConfig.fromJson(json);
}

/// @nodoc
mixin _$ServerToolConfig {
  ToolEnabledConfig get shell => throw _privateConstructorUsedError;
  ToolEnabledConfig get web => throw _privateConstructorUsedError;
  ToolEnabledConfig get cron => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ServerToolConfigCopyWith<ServerToolConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ServerToolConfigCopyWith<$Res> {
  factory $ServerToolConfigCopyWith(
          ServerToolConfig value, $Res Function(ServerToolConfig) then) =
      _$ServerToolConfigCopyWithImpl<$Res, ServerToolConfig>;
  @useResult
  $Res call(
      {ToolEnabledConfig shell, ToolEnabledConfig web, ToolEnabledConfig cron});

  $ToolEnabledConfigCopyWith<$Res> get shell;
  $ToolEnabledConfigCopyWith<$Res> get web;
  $ToolEnabledConfigCopyWith<$Res> get cron;
}

/// @nodoc
class _$ServerToolConfigCopyWithImpl<$Res, $Val extends ServerToolConfig>
    implements $ServerToolConfigCopyWith<$Res> {
  _$ServerToolConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? shell = null,
    Object? web = null,
    Object? cron = null,
  }) {
    return _then(_value.copyWith(
      shell: null == shell
          ? _value.shell
          : shell // ignore: cast_nullable_to_non_nullable
              as ToolEnabledConfig,
      web: null == web
          ? _value.web
          : web // ignore: cast_nullable_to_non_nullable
              as ToolEnabledConfig,
      cron: null == cron
          ? _value.cron
          : cron // ignore: cast_nullable_to_non_nullable
              as ToolEnabledConfig,
    ) as $Val);
  }

  @override
  @pragma('vm:prefer-inline')
  $ToolEnabledConfigCopyWith<$Res> get shell {
    return $ToolEnabledConfigCopyWith<$Res>(_value.shell, (value) {
      return _then(_value.copyWith(shell: value) as $Val);
    });
  }

  @override
  @pragma('vm:prefer-inline')
  $ToolEnabledConfigCopyWith<$Res> get web {
    return $ToolEnabledConfigCopyWith<$Res>(_value.web, (value) {
      return _then(_value.copyWith(web: value) as $Val);
    });
  }

  @override
  @pragma('vm:prefer-inline')
  $ToolEnabledConfigCopyWith<$Res> get cron {
    return $ToolEnabledConfigCopyWith<$Res>(_value.cron, (value) {
      return _then(_value.copyWith(cron: value) as $Val);
    });
  }
}

/// @nodoc
abstract class _$$ServerToolConfigImplCopyWith<$Res>
    implements $ServerToolConfigCopyWith<$Res> {
  factory _$$ServerToolConfigImplCopyWith(_$ServerToolConfigImpl value,
          $Res Function(_$ServerToolConfigImpl) then) =
      __$$ServerToolConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call(
      {ToolEnabledConfig shell, ToolEnabledConfig web, ToolEnabledConfig cron});

  @override
  $ToolEnabledConfigCopyWith<$Res> get shell;
  @override
  $ToolEnabledConfigCopyWith<$Res> get web;
  @override
  $ToolEnabledConfigCopyWith<$Res> get cron;
}

/// @nodoc
class __$$ServerToolConfigImplCopyWithImpl<$Res>
    extends _$ServerToolConfigCopyWithImpl<$Res, _$ServerToolConfigImpl>
    implements _$$ServerToolConfigImplCopyWith<$Res> {
  __$$ServerToolConfigImplCopyWithImpl(_$ServerToolConfigImpl _value,
      $Res Function(_$ServerToolConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? shell = null,
    Object? web = null,
    Object? cron = null,
  }) {
    return _then(_$ServerToolConfigImpl(
      shell: null == shell
          ? _value.shell
          : shell // ignore: cast_nullable_to_non_nullable
              as ToolEnabledConfig,
      web: null == web
          ? _value.web
          : web // ignore: cast_nullable_to_non_nullable
              as ToolEnabledConfig,
      cron: null == cron
          ? _value.cron
          : cron // ignore: cast_nullable_to_non_nullable
              as ToolEnabledConfig,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$ServerToolConfigImpl implements _ServerToolConfig {
  const _$ServerToolConfigImpl(
      {this.shell = const ToolEnabledConfig(),
      this.web = const ToolEnabledConfig(),
      this.cron = const ToolEnabledConfig()});

  factory _$ServerToolConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ServerToolConfigImplFromJson(json);

  @override
  @JsonKey()
  final ToolEnabledConfig shell;
  @override
  @JsonKey()
  final ToolEnabledConfig web;
  @override
  @JsonKey()
  final ToolEnabledConfig cron;

  @override
  String toString() {
    return 'ServerToolConfig(shell: $shell, web: $web, cron: $cron)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ServerToolConfigImpl &&
            (identical(other.shell, shell) || other.shell == shell) &&
            (identical(other.web, web) || other.web == web) &&
            (identical(other.cron, cron) || other.cron == cron));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, shell, web, cron);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ServerToolConfigImplCopyWith<_$ServerToolConfigImpl> get copyWith =>
      __$$ServerToolConfigImplCopyWithImpl<_$ServerToolConfigImpl>(
          this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ServerToolConfigImplToJson(
      this,
    );
  }
}

abstract class _ServerToolConfig implements ServerToolConfig {
  const factory _ServerToolConfig(
      {final ToolEnabledConfig shell,
      final ToolEnabledConfig web,
      final ToolEnabledConfig cron}) = _$ServerToolConfigImpl;

  factory _ServerToolConfig.fromJson(Map<String, dynamic> json) =
      _$ServerToolConfigImpl.fromJson;

  @override
  ToolEnabledConfig get shell;
  @override
  ToolEnabledConfig get web;
  @override
  ToolEnabledConfig get cron;
  @override
  @JsonKey(ignore: true)
  _$$ServerToolConfigImplCopyWith<_$ServerToolConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

ToolEnabledConfig _$ToolEnabledConfigFromJson(Map<String, dynamic> json) {
  return _ToolEnabledConfig.fromJson(json);
}

/// @nodoc
mixin _$ToolEnabledConfig {
  bool get enabled => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ToolEnabledConfigCopyWith<ToolEnabledConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ToolEnabledConfigCopyWith<$Res> {
  factory $ToolEnabledConfigCopyWith(
          ToolEnabledConfig value, $Res Function(ToolEnabledConfig) then) =
      _$ToolEnabledConfigCopyWithImpl<$Res, ToolEnabledConfig>;
  @useResult
  $Res call({bool enabled});
}

/// @nodoc
class _$ToolEnabledConfigCopyWithImpl<$Res, $Val extends ToolEnabledConfig>
    implements $ToolEnabledConfigCopyWith<$Res> {
  _$ToolEnabledConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? enabled = null,
  }) {
    return _then(_value.copyWith(
      enabled: null == enabled
          ? _value.enabled
          : enabled // ignore: cast_nullable_to_non_nullable
              as bool,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$ToolEnabledConfigImplCopyWith<$Res>
    implements $ToolEnabledConfigCopyWith<$Res> {
  factory _$$ToolEnabledConfigImplCopyWith(_$ToolEnabledConfigImpl value,
          $Res Function(_$ToolEnabledConfigImpl) then) =
      __$$ToolEnabledConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call({bool enabled});
}

/// @nodoc
class __$$ToolEnabledConfigImplCopyWithImpl<$Res>
    extends _$ToolEnabledConfigCopyWithImpl<$Res, _$ToolEnabledConfigImpl>
    implements _$$ToolEnabledConfigImplCopyWith<$Res> {
  __$$ToolEnabledConfigImplCopyWithImpl(_$ToolEnabledConfigImpl _value,
      $Res Function(_$ToolEnabledConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? enabled = null,
  }) {
    return _then(_$ToolEnabledConfigImpl(
      enabled: null == enabled
          ? _value.enabled
          : enabled // ignore: cast_nullable_to_non_nullable
              as bool,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$ToolEnabledConfigImpl implements _ToolEnabledConfig {
  const _$ToolEnabledConfigImpl({this.enabled = true});

  factory _$ToolEnabledConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ToolEnabledConfigImplFromJson(json);

  @override
  @JsonKey()
  final bool enabled;

  @override
  String toString() {
    return 'ToolEnabledConfig(enabled: $enabled)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ToolEnabledConfigImpl &&
            (identical(other.enabled, enabled) || other.enabled == enabled));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, enabled);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ToolEnabledConfigImplCopyWith<_$ToolEnabledConfigImpl> get copyWith =>
      __$$ToolEnabledConfigImplCopyWithImpl<_$ToolEnabledConfigImpl>(
          this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ToolEnabledConfigImplToJson(
      this,
    );
  }
}

abstract class _ToolEnabledConfig implements ToolEnabledConfig {
  const factory _ToolEnabledConfig({final bool enabled}) =
      _$ToolEnabledConfigImpl;

  factory _ToolEnabledConfig.fromJson(Map<String, dynamic> json) =
      _$ToolEnabledConfigImpl.fromJson;

  @override
  bool get enabled;
  @override
  @JsonKey(ignore: true)
  _$$ToolEnabledConfigImplCopyWith<_$ToolEnabledConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}
