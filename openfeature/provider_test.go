package openfeature

import (
	"context"
	"fmt"
	"testing"

	"github.com/mixpanel/mixpanel-go/v2/flags"
	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
)

// mockFlagsProvider implements FlagsProvider + AreFlagsReady (simulates LocalFeatureFlagsProvider).
type mockFlagsProvider struct {
	variants map[string]flags.SelectedVariant
	ready    bool
}

func (m *mockFlagsProvider) GetVariantValue(ctx context.Context, flagKey string, fallbackValue any, flagContext flags.FlagContext) (any, error) {
	v, ok := m.variants[flagKey]
	if !ok {
		return fallbackValue, nil
	}
	return v.VariantValue, nil
}

func (m *mockFlagsProvider) GetVariant(ctx context.Context, flagKey string, fallbackVariant flags.SelectedVariant, flagContext flags.FlagContext, reportExposure bool) (flags.SelectedVariant, error) {
	v, ok := m.variants[flagKey]
	if !ok {
		return fallbackVariant, nil
	}
	return v, nil
}

func (m *mockFlagsProvider) AreFlagsReady() bool {
	return m.ready
}

// mockRemoteFlagsProvider implements FlagsProvider without AreFlagsReady (simulates RemoteFeatureFlagsProvider).
type mockRemoteFlagsProvider struct {
	variants map[string]flags.SelectedVariant
}

func (m *mockRemoteFlagsProvider) GetVariantValue(ctx context.Context, flagKey string, fallbackValue any, flagContext flags.FlagContext) (any, error) {
	v, ok := m.variants[flagKey]
	if !ok {
		return fallbackValue, nil
	}
	return v.VariantValue, nil
}

func (m *mockRemoteFlagsProvider) GetVariant(ctx context.Context, flagKey string, fallbackVariant flags.SelectedVariant, flagContext flags.FlagContext, reportExposure bool) (flags.SelectedVariant, error) {
	v, ok := m.variants[flagKey]
	if !ok {
		return fallbackVariant, nil
	}
	return v, nil
}

func strPtr(s string) *string { return &s }

func TestMetadata(t *testing.T) {
	p := NewProvider(&mockFlagsProvider{ready: true})
	assert.Equal(t, "mixpanel-provider", p.Metadata().Name)
}

func TestHooksReturnsNil(t *testing.T) {
	p := NewProvider(&mockFlagsProvider{ready: true})
	assert.Nil(t, p.Hooks())
}

func TestBooleanEvaluation(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"bool-flag": {VariantKey: strPtr("on"), VariantValue: true},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()
	evalCtx := of.FlattenedContext{"distinct_id": "user1"}

	result := p.BooleanEvaluation(ctx, "bool-flag", false, evalCtx)
	assert.Equal(t, true, result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
	assert.Equal(t, "on", result.Variant)
}

func TestBooleanEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"string-flag": {VariantKey: strPtr("variant-a"), VariantValue: "not-a-bool"},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.BooleanEvaluation(ctx, "string-flag", false, nil)
	assert.Equal(t, false, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "TYPE_MISMATCH")
}

func TestStringEvaluation(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"str-flag": {VariantKey: strPtr("variant-a"), VariantValue: "hello"},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "str-flag", "default", nil)
	assert.Equal(t, "hello", result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
	assert.Equal(t, "variant-a", result.Variant)
}

func TestStringEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"bool-flag": {VariantKey: strPtr("on"), VariantValue: true},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "bool-flag", "default", nil)
	assert.Equal(t, "default", result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "TYPE_MISMATCH")
}

func TestFloatEvaluation(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"float-flag": {VariantKey: strPtr("half"), VariantValue: 0.5},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.FloatEvaluation(ctx, "float-flag", 0.0, nil)
	assert.Equal(t, 0.5, result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
	assert.Equal(t, "half", result.Variant)
}

func TestFloatEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"str-flag": {VariantKey: strPtr("v"), VariantValue: "not-a-float"},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.FloatEvaluation(ctx, "str-flag", 1.0, nil)
	assert.Equal(t, 1.0, result.Value)
	assert.Contains(t, result.ResolutionError.Error(), "TYPE_MISMATCH")
}

func TestIntEvaluation(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"int-flag": {VariantKey: strPtr("big"), VariantValue: float64(42)},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "int-flag", 0, nil)
	assert.Equal(t, int64(42), result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
	assert.Equal(t, "big", result.Variant)
}

func TestIntEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"str-flag": {VariantKey: strPtr("v"), VariantValue: "not-an-int"},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "str-flag", 0, nil)
	assert.Equal(t, int64(0), result.Value)
	assert.Contains(t, result.ResolutionError.Error(), "TYPE_MISMATCH")
}

func TestObjectEvaluation(t *testing.T) {
	obj := map[string]any{"key": "value"}
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"obj-flag": {VariantKey: strPtr("config"), VariantValue: obj},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.ObjectEvaluation(ctx, "obj-flag", nil, nil)
	assert.Equal(t, obj, result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
	assert.Equal(t, "config", result.Variant)
}

func TestFlagNotFound(t *testing.T) {
	mock := &mockFlagsProvider{
		ready:    true,
		variants: map[string]flags.SelectedVariant{},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.BooleanEvaluation(ctx, "missing-flag", false, nil)
	assert.Equal(t, false, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "FLAG_NOT_FOUND")
}

func TestProviderNotReady(t *testing.T) {
	mock := &mockFlagsProvider{
		ready:    false,
		variants: map[string]flags.SelectedVariant{},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "any-flag", "default", nil)
	assert.Equal(t, "default", result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "PROVIDER_NOT_READY")
}

func TestContextPassedThrough(t *testing.T) {
	// Use a custom mock that captures the flag context
	var capturedContext flags.FlagContext
	captureMock := &contextCaptureMock{
		ready:           true,
		capturedContext: &capturedContext,
		variant:         flags.SelectedVariant{VariantKey: strPtr("v"), VariantValue: true},
	}
	p := NewProvider(captureMock)
	ctx := context.Background()

	evalCtx := of.FlattenedContext{
		"distinct_id":  "user123",
		"targetingKey": "some-key",
		"plan":         "premium",
	}

	p.BooleanEvaluation(ctx, "flag", false, evalCtx)

	assert.Equal(t, "user123", capturedContext["distinct_id"])
	assert.Equal(t, "some-key", capturedContext["targetingKey"])
	assert.Equal(t, "premium", capturedContext["plan"])
}

func TestDefaultValueReturnedOnError(t *testing.T) {
	mock := &mockFlagsProvider{
		ready:    false,
		variants: map[string]flags.SelectedVariant{},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	boolResult := p.BooleanEvaluation(ctx, "f", true, nil)
	assert.Equal(t, true, boolResult.Value)

	strResult := p.StringEvaluation(ctx, "f", "mydefault", nil)
	assert.Equal(t, "mydefault", strResult.Value)

	floatResult := p.FloatEvaluation(ctx, "f", 3.14, nil)
	assert.Equal(t, 3.14, floatResult.Value)

	intResult := p.IntEvaluation(ctx, "f", 99, nil)
	assert.Equal(t, int64(99), intResult.Value)

	objResult := p.ObjectEvaluation(ctx, "f", "objdefault", nil)
	assert.Equal(t, "objdefault", objResult.Value)
}

func TestTargetingKeyNotSpecial(t *testing.T) {
	var capturedContext flags.FlagContext
	captureMock := &contextCaptureMock{
		ready:           true,
		capturedContext: &capturedContext,
		variant:         flags.SelectedVariant{VariantKey: strPtr("v"), VariantValue: "val"},
	}
	p := NewProvider(captureMock)
	ctx := context.Background()

	evalCtx := of.FlattenedContext{
		"targetingKey": "targeting-value",
		"distinct_id":  "actual-user",
	}

	p.StringEvaluation(ctx, "flag", "", evalCtx)

	// targetingKey should just be passed through as-is, not mapped to distinct_id
	assert.Equal(t, "targeting-value", capturedContext["targetingKey"])
	assert.Equal(t, "actual-user", capturedContext["distinct_id"])
}

func TestRemoteProviderSkipsReadinessCheck(t *testing.T) {
	mock := &mockRemoteFlagsProvider{
		variants: map[string]flags.SelectedVariant{
			"remote-flag": {VariantKey: strPtr("v1"), VariantValue: "remote-value"},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "remote-flag", "default", nil)
	assert.Equal(t, "remote-value", result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
}

func TestShutdownIsNoOp(t *testing.T) {
	p := NewProvider(&mockFlagsProvider{ready: true})
	// Should not panic
	p.Shutdown()
}

// contextCaptureMock captures the FlagContext passed to GetVariant.
type contextCaptureMock struct {
	ready           bool
	capturedContext *flags.FlagContext
	variant         flags.SelectedVariant
}

func (m *contextCaptureMock) GetVariantValue(ctx context.Context, flagKey string, fallbackValue any, flagContext flags.FlagContext) (any, error) {
	*m.capturedContext = flagContext
	return m.variant.VariantValue, nil
}

func (m *contextCaptureMock) GetVariant(ctx context.Context, flagKey string, fallbackVariant flags.SelectedVariant, flagContext flags.FlagContext, reportExposure bool) (flags.SelectedVariant, error) {
	*m.capturedContext = flagContext
	return m.variant, nil
}

func (m *contextCaptureMock) AreFlagsReady() bool {
	return m.ready
}

// errorFlagsProvider returns an error from GetVariant.
type errorFlagsProvider struct {
	ready bool
}

func (m *errorFlagsProvider) GetVariantValue(ctx context.Context, flagKey string, fallbackValue any, flagContext flags.FlagContext) (any, error) {
	return fallbackValue, fmt.Errorf("sdk error")
}

func (m *errorFlagsProvider) GetVariant(ctx context.Context, flagKey string, fallbackVariant flags.SelectedVariant, flagContext flags.FlagContext, reportExposure bool) (flags.SelectedVariant, error) {
	return fallbackVariant, fmt.Errorf("sdk error")
}

func (m *errorFlagsProvider) AreFlagsReady() bool {
	return m.ready
}

func TestSDKErrorReturnsDefault(t *testing.T) {
	mock := &errorFlagsProvider{ready: true}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.BooleanEvaluation(ctx, "flag", true, nil)
	assert.Equal(t, true, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "GENERAL")
}

func TestFloatEvaluationFromInt(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"int-as-float": {VariantKey: strPtr("v1"), VariantValue: int(42)},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.FloatEvaluation(ctx, "int-as-float", 0.0, nil)
	assert.Equal(t, 42.0, result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
}

func TestIntEvaluationFromNativeInt64(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"int64-flag": {VariantKey: strPtr("limit"), VariantValue: int64(100)},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "int64-flag", 0, nil)
	assert.Equal(t, int64(100), result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
}

func TestIntEvaluationNonWholeFloat(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"pi": {VariantKey: strPtr("v"), VariantValue: 3.14},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "pi", 0, nil)
	assert.Equal(t, int64(0), result.Value)
	assert.Contains(t, result.ResolutionError.Error(), "TYPE_MISMATCH")
}

func TestFlagNotFoundString(t *testing.T) {
	mock := &mockFlagsProvider{ready: true, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "missing-flag", "fallback", nil)
	assert.Equal(t, "fallback", result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "FLAG_NOT_FOUND")
}

func TestFlagNotFoundFloat(t *testing.T) {
	mock := &mockFlagsProvider{ready: true, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.FloatEvaluation(ctx, "missing-flag", 1.5, nil)
	assert.Equal(t, 1.5, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "FLAG_NOT_FOUND")
}

func TestFlagNotFoundInt(t *testing.T) {
	mock := &mockFlagsProvider{ready: true, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "missing-flag", 7, nil)
	assert.Equal(t, int64(7), result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "FLAG_NOT_FOUND")
}

func TestFlagNotFoundObject(t *testing.T) {
	mock := &mockFlagsProvider{ready: true, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	defaultObj := map[string]any{"default": true}
	result := p.ObjectEvaluation(ctx, "missing-flag", defaultObj, nil)
	assert.Equal(t, defaultObj, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "FLAG_NOT_FOUND")
}

func TestProviderNotReadyBoolean(t *testing.T) {
	mock := &mockFlagsProvider{ready: false, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.BooleanEvaluation(ctx, "any-flag", true, nil)
	assert.Equal(t, true, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "PROVIDER_NOT_READY")
}

func TestProviderNotReadyFloat(t *testing.T) {
	mock := &mockFlagsProvider{ready: false, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.FloatEvaluation(ctx, "any-flag", 2.5, nil)
	assert.Equal(t, 2.5, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "PROVIDER_NOT_READY")
}

func TestProviderNotReadyInt(t *testing.T) {
	mock := &mockFlagsProvider{ready: false, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "any-flag", 10, nil)
	assert.Equal(t, int64(10), result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "PROVIDER_NOT_READY")
}

func TestProviderNotReadyObject(t *testing.T) {
	mock := &mockFlagsProvider{ready: false, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	defaultObj := map[string]any{"fallback": true}
	result := p.ObjectEvaluation(ctx, "any-flag", defaultObj, nil)
	assert.Equal(t, defaultObj, result.Value)
	assert.Equal(t, of.ErrorReason, result.Reason)
	assert.Contains(t, result.ResolutionError.Error(), "PROVIDER_NOT_READY")
}

func TestNullVariantKeyReturnsFlagNotFound(t *testing.T) {
	// When VariantKey is nil, the provider should treat it as flag not found.
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"nil-variant-flag": {VariantKey: nil, VariantValue: "some-value"},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	boolResult := p.BooleanEvaluation(ctx, "nil-variant-flag", false, nil)
	assert.Equal(t, false, boolResult.Value)
	assert.Equal(t, of.ErrorReason, boolResult.Reason)
	assert.Contains(t, boolResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	strResult := p.StringEvaluation(ctx, "nil-variant-flag", "default", nil)
	assert.Equal(t, "default", strResult.Value)
	assert.Equal(t, of.ErrorReason, strResult.Reason)
	assert.Contains(t, strResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	floatResult := p.FloatEvaluation(ctx, "nil-variant-flag", 1.0, nil)
	assert.Equal(t, 1.0, floatResult.Value)
	assert.Equal(t, of.ErrorReason, floatResult.Reason)
	assert.Contains(t, floatResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	intResult := p.IntEvaluation(ctx, "nil-variant-flag", 5, nil)
	assert.Equal(t, int64(5), intResult.Value)
	assert.Equal(t, of.ErrorReason, intResult.Reason)
	assert.Contains(t, intResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	objResult := p.ObjectEvaluation(ctx, "nil-variant-flag", nil, nil)
	assert.Nil(t, objResult.Value)
	assert.Equal(t, of.ErrorReason, objResult.Reason)
	assert.Contains(t, objResult.ResolutionError.Error(), "FLAG_NOT_FOUND")
}

func TestEmptyVariantKeyIsValid(t *testing.T) {
	// An empty string variant key is still a valid (non-nil) key.
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"empty-key-flag": {VariantKey: strPtr(""), VariantValue: "value"},
		},
	}
	p := NewProvider(mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "empty-key-flag", "default", nil)
	assert.Equal(t, "value", result.Value)
	assert.Equal(t, of.StaticReason, result.Reason)
	assert.Equal(t, "", result.Variant)
}

func TestFlagNotFoundAllTypes(t *testing.T) {
	mock := &mockFlagsProvider{ready: true, variants: map[string]flags.SelectedVariant{}}
	p := NewProvider(mock)
	ctx := context.Background()

	boolResult := p.BooleanEvaluation(ctx, "missing", true, nil)
	assert.Equal(t, true, boolResult.Value)
	assert.Contains(t, boolResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	strResult := p.StringEvaluation(ctx, "missing", "fallback", nil)
	assert.Equal(t, "fallback", strResult.Value)
	assert.Contains(t, strResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	floatResult := p.FloatEvaluation(ctx, "missing", 9.9, nil)
	assert.Equal(t, 9.9, floatResult.Value)
	assert.Contains(t, floatResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	intResult := p.IntEvaluation(ctx, "missing", 42, nil)
	assert.Equal(t, int64(42), intResult.Value)
	assert.Contains(t, intResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	objResult := p.ObjectEvaluation(ctx, "missing", map[string]any{"d": 1}, nil)
	assert.Equal(t, map[string]any{"d": 1}, objResult.Value)
	assert.Contains(t, objResult.ResolutionError.Error(), "FLAG_NOT_FOUND")
}
