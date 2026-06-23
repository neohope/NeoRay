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

LLMConfig _$LLMConfigFromJson(Map<String, dynamic> json) {
  return _LLMConfig.fromJson(json);
}

/// @nodoc
mixin _$LLMConfig {
  String get provider => throw _privateConstructorUsedError;
  String get apiKey => throw _privateConstructorUsedError;
  String get apiUrl => throw _privateConstructorUsedError;
  String get model => throw _privateConstructorUsedError;
  int get maxTokens => throw _privateConstructorUsedError;
  double get temperature => throw _privateConstructorUsedError;
  int get timeout => throw _privateConstructorUsedError;
  String get reasoningEffort => throw _privateConstructorUsedError;
  bool get promptCacheEnabled => throw _privateConstructorUsedError;
  List<FallbackModelConfig> get fallbackModels =>
      throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $LLMConfigCopyWith<LLMConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $LLMConfigCopyWith<$Res> {
  factory $LLMConfigCopyWith(LLMConfig value, $Res Function(LLMConfig) then) =
      _$LLMConfigCopyWithImpl<$Res, LLMConfig>;
  @useResult
  $Res call(
      {String provider,
      String apiKey,
      String apiUrl,
      String model,
      int maxTokens,
      double temperature,
      int timeout,
      String reasoningEffort,
      bool promptCacheEnabled,
      List<FallbackModelConfig> fallbackModels});
}

/// @nodoc
class _$LLMConfigCopyWithImpl<$Res, $Val extends LLMConfig>
    implements $LLMConfigCopyWith<$Res> {
  _$LLMConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? provider = null,
    Object? apiKey = null,
    Object? apiUrl = null,
    Object? model = null,
    Object? maxTokens = null,
    Object? temperature = null,
    Object? timeout = null,
    Object? reasoningEffort = null,
    Object? promptCacheEnabled = null,
    Object? fallbackModels = null,
  }) {
    return _then(_value.copyWith(
      provider: null == provider
          ? _value.provider
          : provider // ignore: cast_nullable_to_non_nullable
              as String,
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
              as int,
      reasoningEffort: null == reasoningEffort
          ? _value.reasoningEffort
          : reasoningEffort // ignore: cast_nullable_to_non_nullable
              as String,
      promptCacheEnabled: null == promptCacheEnabled
          ? _value.promptCacheEnabled
          : promptCacheEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
      fallbackModels: null == fallbackModels
          ? _value.fallbackModels
          : fallbackModels // ignore: cast_nullable_to_non_nullable
              as List<FallbackModelConfig>,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$LLMConfigImplCopyWith<$Res>
    implements $LLMConfigCopyWith<$Res> {
  factory _$$LLMConfigImplCopyWith(
          _$LLMConfigImpl value, $Res Function(_$LLMConfigImpl) then) =
      __$$LLMConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call(
      {String provider,
      String apiKey,
      String apiUrl,
      String model,
      int maxTokens,
      double temperature,
      int timeout,
      String reasoningEffort,
      bool promptCacheEnabled,
      List<FallbackModelConfig> fallbackModels});
}

/// @nodoc
class __$$LLMConfigImplCopyWithImpl<$Res>
    extends _$LLMConfigCopyWithImpl<$Res, _$LLMConfigImpl>
    implements _$$LLMConfigImplCopyWith<$Res> {
  __$$LLMConfigImplCopyWithImpl(
      _$LLMConfigImpl _value, $Res Function(_$LLMConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? provider = null,
    Object? apiKey = null,
    Object? apiUrl = null,
    Object? model = null,
    Object? maxTokens = null,
    Object? temperature = null,
    Object? timeout = null,
    Object? reasoningEffort = null,
    Object? promptCacheEnabled = null,
    Object? fallbackModels = null,
  }) {
    return _then(_$LLMConfigImpl(
      provider: null == provider
          ? _value.provider
          : provider // ignore: cast_nullable_to_non_nullable
              as String,
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
              as int,
      reasoningEffort: null == reasoningEffort
          ? _value.reasoningEffort
          : reasoningEffort // ignore: cast_nullable_to_non_nullable
              as String,
      promptCacheEnabled: null == promptCacheEnabled
          ? _value.promptCacheEnabled
          : promptCacheEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
      fallbackModels: null == fallbackModels
          ? _value._fallbackModels
          : fallbackModels // ignore: cast_nullable_to_non_nullable
              as List<FallbackModelConfig>,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$LLMConfigImpl implements _LLMConfig {
  const _$LLMConfigImpl(
      {this.provider = 'openai',
      this.apiKey = '',
      this.apiUrl = 'https://api.openai.com/v1',
      this.model = 'gpt-4',
      this.maxTokens = 4096,
      this.temperature = 0.7,
      this.timeout = 120,
      this.reasoningEffort = 'none',
      this.promptCacheEnabled = false,
      final List<FallbackModelConfig> fallbackModels = const []})
      : _fallbackModels = fallbackModels;

  factory _$LLMConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$LLMConfigImplFromJson(json);

  @override
  @JsonKey()
  final String provider;
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
  final int timeout;
  @override
  @JsonKey()
  final String reasoningEffort;
  @override
  @JsonKey()
  final bool promptCacheEnabled;
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
    return 'LLMConfig(provider: $provider, apiKey: $apiKey, apiUrl: $apiUrl, model: $model, maxTokens: $maxTokens, temperature: $temperature, timeout: $timeout, reasoningEffort: $reasoningEffort, promptCacheEnabled: $promptCacheEnabled, fallbackModels: $fallbackModels)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$LLMConfigImpl &&
            (identical(other.provider, provider) ||
                other.provider == provider) &&
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
                other.promptCacheEnabled == promptCacheEnabled) &&
            const DeepCollectionEquality()
                .equals(other._fallbackModels, _fallbackModels));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(
      runtimeType,
      provider,
      apiKey,
      apiUrl,
      model,
      maxTokens,
      temperature,
      timeout,
      reasoningEffort,
      promptCacheEnabled,
      const DeepCollectionEquality().hash(_fallbackModels));

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$LLMConfigImplCopyWith<_$LLMConfigImpl> get copyWith =>
      __$$LLMConfigImplCopyWithImpl<_$LLMConfigImpl>(this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$LLMConfigImplToJson(
      this,
    );
  }
}

abstract class _LLMConfig implements LLMConfig {
  const factory _LLMConfig(
      {final String provider,
      final String apiKey,
      final String apiUrl,
      final String model,
      final int maxTokens,
      final double temperature,
      final int timeout,
      final String reasoningEffort,
      final bool promptCacheEnabled,
      final List<FallbackModelConfig> fallbackModels}) = _$LLMConfigImpl;

  factory _LLMConfig.fromJson(Map<String, dynamic> json) =
      _$LLMConfigImpl.fromJson;

  @override
  String get provider;
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
  int get timeout;
  @override
  String get reasoningEffort;
  @override
  bool get promptCacheEnabled;
  @override
  List<FallbackModelConfig> get fallbackModels;
  @override
  @JsonKey(ignore: true)
  _$$LLMConfigImplCopyWith<_$LLMConfigImpl> get copyWith =>
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

ChannelConfig _$ChannelConfigFromJson(Map<String, dynamic> json) {
  return _ChannelConfig.fromJson(json);
}

/// @nodoc
mixin _$ChannelConfig {
  bool get enabled => throw _privateConstructorUsedError;
  String get provider => throw _privateConstructorUsedError;
  String get appId => throw _privateConstructorUsedError;
  String get appSecret => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ChannelConfigCopyWith<ChannelConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ChannelConfigCopyWith<$Res> {
  factory $ChannelConfigCopyWith(
          ChannelConfig value, $Res Function(ChannelConfig) then) =
      _$ChannelConfigCopyWithImpl<$Res, ChannelConfig>;
  @useResult
  $Res call({bool enabled, String provider, String appId, String appSecret});
}

/// @nodoc
class _$ChannelConfigCopyWithImpl<$Res, $Val extends ChannelConfig>
    implements $ChannelConfigCopyWith<$Res> {
  _$ChannelConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? enabled = null,
    Object? provider = null,
    Object? appId = null,
    Object? appSecret = null,
  }) {
    return _then(_value.copyWith(
      enabled: null == enabled
          ? _value.enabled
          : enabled // ignore: cast_nullable_to_non_nullable
              as bool,
      provider: null == provider
          ? _value.provider
          : provider // ignore: cast_nullable_to_non_nullable
              as String,
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
abstract class _$$ChannelConfigImplCopyWith<$Res>
    implements $ChannelConfigCopyWith<$Res> {
  factory _$$ChannelConfigImplCopyWith(
          _$ChannelConfigImpl value, $Res Function(_$ChannelConfigImpl) then) =
      __$$ChannelConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call({bool enabled, String provider, String appId, String appSecret});
}

/// @nodoc
class __$$ChannelConfigImplCopyWithImpl<$Res>
    extends _$ChannelConfigCopyWithImpl<$Res, _$ChannelConfigImpl>
    implements _$$ChannelConfigImplCopyWith<$Res> {
  __$$ChannelConfigImplCopyWithImpl(
      _$ChannelConfigImpl _value, $Res Function(_$ChannelConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? enabled = null,
    Object? provider = null,
    Object? appId = null,
    Object? appSecret = null,
  }) {
    return _then(_$ChannelConfigImpl(
      enabled: null == enabled
          ? _value.enabled
          : enabled // ignore: cast_nullable_to_non_nullable
              as bool,
      provider: null == provider
          ? _value.provider
          : provider // ignore: cast_nullable_to_non_nullable
              as String,
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
class _$ChannelConfigImpl implements _ChannelConfig {
  const _$ChannelConfigImpl(
      {this.enabled = false,
      this.provider = 'feishu',
      this.appId = '',
      this.appSecret = ''});

  factory _$ChannelConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ChannelConfigImplFromJson(json);

  @override
  @JsonKey()
  final bool enabled;
  @override
  @JsonKey()
  final String provider;
  @override
  @JsonKey()
  final String appId;
  @override
  @JsonKey()
  final String appSecret;

  @override
  String toString() {
    return 'ChannelConfig(enabled: $enabled, provider: $provider, appId: $appId, appSecret: $appSecret)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ChannelConfigImpl &&
            (identical(other.enabled, enabled) || other.enabled == enabled) &&
            (identical(other.provider, provider) ||
                other.provider == provider) &&
            (identical(other.appId, appId) || other.appId == appId) &&
            (identical(other.appSecret, appSecret) ||
                other.appSecret == appSecret));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode =>
      Object.hash(runtimeType, enabled, provider, appId, appSecret);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ChannelConfigImplCopyWith<_$ChannelConfigImpl> get copyWith =>
      __$$ChannelConfigImplCopyWithImpl<_$ChannelConfigImpl>(this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ChannelConfigImplToJson(
      this,
    );
  }
}

abstract class _ChannelConfig implements ChannelConfig {
  const factory _ChannelConfig(
      {final bool enabled,
      final String provider,
      final String appId,
      final String appSecret}) = _$ChannelConfigImpl;

  factory _ChannelConfig.fromJson(Map<String, dynamic> json) =
      _$ChannelConfigImpl.fromJson;

  @override
  bool get enabled;
  @override
  String get provider;
  @override
  String get appId;
  @override
  String get appSecret;
  @override
  @JsonKey(ignore: true)
  _$$ChannelConfigImplCopyWith<_$ChannelConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

ToolConfig _$ToolConfigFromJson(Map<String, dynamic> json) {
  return _ToolConfig.fromJson(json);
}

/// @nodoc
mixin _$ToolConfig {
  bool get shellEnabled => throw _privateConstructorUsedError;
  bool get cronEnabled => throw _privateConstructorUsedError;
  bool get webEnabled => throw _privateConstructorUsedError;

  Map<String, dynamic> toJson() => throw _privateConstructorUsedError;
  @JsonKey(ignore: true)
  $ToolConfigCopyWith<ToolConfig> get copyWith =>
      throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $ToolConfigCopyWith<$Res> {
  factory $ToolConfigCopyWith(
          ToolConfig value, $Res Function(ToolConfig) then) =
      _$ToolConfigCopyWithImpl<$Res, ToolConfig>;
  @useResult
  $Res call({bool shellEnabled, bool cronEnabled, bool webEnabled});
}

/// @nodoc
class _$ToolConfigCopyWithImpl<$Res, $Val extends ToolConfig>
    implements $ToolConfigCopyWith<$Res> {
  _$ToolConfigCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? shellEnabled = null,
    Object? cronEnabled = null,
    Object? webEnabled = null,
  }) {
    return _then(_value.copyWith(
      shellEnabled: null == shellEnabled
          ? _value.shellEnabled
          : shellEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
      cronEnabled: null == cronEnabled
          ? _value.cronEnabled
          : cronEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
      webEnabled: null == webEnabled
          ? _value.webEnabled
          : webEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
    ) as $Val);
  }
}

/// @nodoc
abstract class _$$ToolConfigImplCopyWith<$Res>
    implements $ToolConfigCopyWith<$Res> {
  factory _$$ToolConfigImplCopyWith(
          _$ToolConfigImpl value, $Res Function(_$ToolConfigImpl) then) =
      __$$ToolConfigImplCopyWithImpl<$Res>;
  @override
  @useResult
  $Res call({bool shellEnabled, bool cronEnabled, bool webEnabled});
}

/// @nodoc
class __$$ToolConfigImplCopyWithImpl<$Res>
    extends _$ToolConfigCopyWithImpl<$Res, _$ToolConfigImpl>
    implements _$$ToolConfigImplCopyWith<$Res> {
  __$$ToolConfigImplCopyWithImpl(
      _$ToolConfigImpl _value, $Res Function(_$ToolConfigImpl) _then)
      : super(_value, _then);

  @pragma('vm:prefer-inline')
  @override
  $Res call({
    Object? shellEnabled = null,
    Object? cronEnabled = null,
    Object? webEnabled = null,
  }) {
    return _then(_$ToolConfigImpl(
      shellEnabled: null == shellEnabled
          ? _value.shellEnabled
          : shellEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
      cronEnabled: null == cronEnabled
          ? _value.cronEnabled
          : cronEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
      webEnabled: null == webEnabled
          ? _value.webEnabled
          : webEnabled // ignore: cast_nullable_to_non_nullable
              as bool,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$ToolConfigImpl implements _ToolConfig {
  const _$ToolConfigImpl(
      {this.shellEnabled = true,
      this.cronEnabled = true,
      this.webEnabled = false});

  factory _$ToolConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$ToolConfigImplFromJson(json);

  @override
  @JsonKey()
  final bool shellEnabled;
  @override
  @JsonKey()
  final bool cronEnabled;
  @override
  @JsonKey()
  final bool webEnabled;

  @override
  String toString() {
    return 'ToolConfig(shellEnabled: $shellEnabled, cronEnabled: $cronEnabled, webEnabled: $webEnabled)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$ToolConfigImpl &&
            (identical(other.shellEnabled, shellEnabled) ||
                other.shellEnabled == shellEnabled) &&
            (identical(other.cronEnabled, cronEnabled) ||
                other.cronEnabled == cronEnabled) &&
            (identical(other.webEnabled, webEnabled) ||
                other.webEnabled == webEnabled));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode =>
      Object.hash(runtimeType, shellEnabled, cronEnabled, webEnabled);

  @JsonKey(ignore: true)
  @override
  @pragma('vm:prefer-inline')
  _$$ToolConfigImplCopyWith<_$ToolConfigImpl> get copyWith =>
      __$$ToolConfigImplCopyWithImpl<_$ToolConfigImpl>(this, _$identity);

  @override
  Map<String, dynamic> toJson() {
    return _$$ToolConfigImplToJson(
      this,
    );
  }
}

abstract class _ToolConfig implements ToolConfig {
  const factory _ToolConfig(
      {final bool shellEnabled,
      final bool cronEnabled,
      final bool webEnabled}) = _$ToolConfigImpl;

  factory _ToolConfig.fromJson(Map<String, dynamic> json) =
      _$ToolConfigImpl.fromJson;

  @override
  bool get shellEnabled;
  @override
  bool get cronEnabled;
  @override
  bool get webEnabled;
  @override
  @JsonKey(ignore: true)
  _$$ToolConfigImplCopyWith<_$ToolConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}

AppConfig _$AppConfigFromJson(Map<String, dynamic> json) {
  return _AppConfig.fromJson(json);
}

/// @nodoc
mixin _$AppConfig {
  String get serverUrl => throw _privateConstructorUsedError;
  LLMConfig get llm => throw _privateConstructorUsedError;
  ChannelConfig get channel => throw _privateConstructorUsedError;
  ToolConfig get tools => throw _privateConstructorUsedError;

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
  $Res call(
      {String serverUrl,
      LLMConfig llm,
      ChannelConfig channel,
      ToolConfig tools});

  $LLMConfigCopyWith<$Res> get llm;
  $ChannelConfigCopyWith<$Res> get channel;
  $ToolConfigCopyWith<$Res> get tools;
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
    Object? llm = null,
    Object? channel = null,
    Object? tools = null,
  }) {
    return _then(_value.copyWith(
      serverUrl: null == serverUrl
          ? _value.serverUrl
          : serverUrl // ignore: cast_nullable_to_non_nullable
              as String,
      llm: null == llm
          ? _value.llm
          : llm // ignore: cast_nullable_to_non_nullable
              as LLMConfig,
      channel: null == channel
          ? _value.channel
          : channel // ignore: cast_nullable_to_non_nullable
              as ChannelConfig,
      tools: null == tools
          ? _value.tools
          : tools // ignore: cast_nullable_to_non_nullable
              as ToolConfig,
    ) as $Val);
  }

  @override
  @pragma('vm:prefer-inline')
  $LLMConfigCopyWith<$Res> get llm {
    return $LLMConfigCopyWith<$Res>(_value.llm, (value) {
      return _then(_value.copyWith(llm: value) as $Val);
    });
  }

  @override
  @pragma('vm:prefer-inline')
  $ChannelConfigCopyWith<$Res> get channel {
    return $ChannelConfigCopyWith<$Res>(_value.channel, (value) {
      return _then(_value.copyWith(channel: value) as $Val);
    });
  }

  @override
  @pragma('vm:prefer-inline')
  $ToolConfigCopyWith<$Res> get tools {
    return $ToolConfigCopyWith<$Res>(_value.tools, (value) {
      return _then(_value.copyWith(tools: value) as $Val);
    });
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
  $Res call(
      {String serverUrl,
      LLMConfig llm,
      ChannelConfig channel,
      ToolConfig tools});

  @override
  $LLMConfigCopyWith<$Res> get llm;
  @override
  $ChannelConfigCopyWith<$Res> get channel;
  @override
  $ToolConfigCopyWith<$Res> get tools;
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
    Object? llm = null,
    Object? channel = null,
    Object? tools = null,
  }) {
    return _then(_$AppConfigImpl(
      serverUrl: null == serverUrl
          ? _value.serverUrl
          : serverUrl // ignore: cast_nullable_to_non_nullable
              as String,
      llm: null == llm
          ? _value.llm
          : llm // ignore: cast_nullable_to_non_nullable
              as LLMConfig,
      channel: null == channel
          ? _value.channel
          : channel // ignore: cast_nullable_to_non_nullable
              as ChannelConfig,
      tools: null == tools
          ? _value.tools
          : tools // ignore: cast_nullable_to_non_nullable
              as ToolConfig,
    ));
  }
}

/// @nodoc
@JsonSerializable()
class _$AppConfigImpl implements _AppConfig {
  const _$AppConfigImpl(
      {this.serverUrl = 'http://localhost:8080',
      this.llm = const LLMConfig(),
      this.channel = const ChannelConfig(),
      this.tools = const ToolConfig()});

  factory _$AppConfigImpl.fromJson(Map<String, dynamic> json) =>
      _$$AppConfigImplFromJson(json);

  @override
  @JsonKey()
  final String serverUrl;
  @override
  @JsonKey()
  final LLMConfig llm;
  @override
  @JsonKey()
  final ChannelConfig channel;
  @override
  @JsonKey()
  final ToolConfig tools;

  @override
  String toString() {
    return 'AppConfig(serverUrl: $serverUrl, llm: $llm, channel: $channel, tools: $tools)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$AppConfigImpl &&
            (identical(other.serverUrl, serverUrl) ||
                other.serverUrl == serverUrl) &&
            (identical(other.llm, llm) || other.llm == llm) &&
            (identical(other.channel, channel) || other.channel == channel) &&
            (identical(other.tools, tools) || other.tools == tools));
  }

  @JsonKey(ignore: true)
  @override
  int get hashCode => Object.hash(runtimeType, serverUrl, llm, channel, tools);

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
  const factory _AppConfig(
      {final String serverUrl,
      final LLMConfig llm,
      final ChannelConfig channel,
      final ToolConfig tools}) = _$AppConfigImpl;

  factory _AppConfig.fromJson(Map<String, dynamic> json) =
      _$AppConfigImpl.fromJson;

  @override
  String get serverUrl;
  @override
  LLMConfig get llm;
  @override
  ChannelConfig get channel;
  @override
  ToolConfig get tools;
  @override
  @JsonKey(ignore: true)
  _$$AppConfigImplCopyWith<_$AppConfigImpl> get copyWith =>
      throw _privateConstructorUsedError;
}
