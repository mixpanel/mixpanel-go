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

func (m *mockFlagsProvider) GetVariant(_ context.Context, flagKey string, fallbackVariant flags.SelectedVariant, _ flags.FlagContext, _ bool) (flags.SelectedVariant, error) {
	return getVariant(m.variants, flagKey, fallbackVariant)
}

func (m *mockFlagsProvider) AreFlagsReady() bool {
	return m.ready
}

// mockRemoteFlagsProvider implements FlagsProvider without AreFlagsReady (simulates RemoteFeatureFlagsProvider).
type mockRemoteFlagsProvider struct {
	variants map[string]flags.SelectedVariant
}

func (m *mockRemoteFlagsProvider) GetVariant(_ context.Context, flagKey string, fallbackVariant flags.SelectedVariant, _ flags.FlagContext, _ bool) (flags.SelectedVariant, error) {
	return getVariant(m.variants, flagKey, fallbackVariant)
}

func strPtr(s string) *string { return &s }

func getVariant(variants map[string]flags.SelectedVariant, flagKey string, fallbackVariant flags.SelectedVariant) (flags.SelectedVariant, error) {
	v, ok := variants[flagKey]
	if !ok {
		return fallbackVariant, nil
	}
	return v, nil
}

// mustNewProvider calls NewProvider and fails the test if it returns an error.
func mustNewProvider(t *testing.T, fp FlagsProvider) *Provider {
	t.Helper()
	p, err := NewProvider(fp)
	assert.NoError(t, err)
	return p
}

func TestNewProviderNilFlagsProvider(t *testing.T) {
	p, err := NewProvider(nil)
	assert.Nil(t, p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flagsProvider must not be nil")
}

func TestMetadata(t *testing.T) {
	p := mustNewProvider(t, &mockFlagsProvider{ready: true})
	assert.Equal(t, "mixpanel-provider", p.Metadata().Name)
}

func TestHooksReturnsNil(t *testing.T) {
	p := mustNewProvider(t, &mockFlagsProvider{ready: true})
	assert.Nil(t, p.Hooks())
}

func TestBooleanEvaluation(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"bool-flag": {VariantKey: strPtr("on"), VariantValue: true},
		},
	}
	p := mustNewProvider(t, mock)
	ctx := context.Background()
	evalCtx := of.FlattenedContext{"distinct_id": "user1"}

	result := p.BooleanEvaluation(ctx, "bool-flag", false, evalCtx)
	assert.Equal(t, true, result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
	assert.Equal(t, "on", result.Variant)
}

func TestBooleanEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"string-flag": {VariantKey: strPtr("variant-a"), VariantValue: "not-a-bool"},
		},
	}
	p := mustNewProvider(t, mock)
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
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "str-flag", "default", nil)
	assert.Equal(t, "hello", result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
	assert.Equal(t, "variant-a", result.Variant)
}

func TestStringEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"bool-flag": {VariantKey: strPtr("on"), VariantValue: true},
		},
	}
	p := mustNewProvider(t, mock)
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
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.FloatEvaluation(ctx, "float-flag", 0.0, nil)
	assert.Equal(t, 0.5, result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
	assert.Equal(t, "half", result.Variant)
}

func TestFloatEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"str-flag": {VariantKey: strPtr("v"), VariantValue: "not-a-float"},
		},
	}
	p := mustNewProvider(t, mock)
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
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "int-flag", 0, nil)
	assert.Equal(t, int64(42), result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
	assert.Equal(t, "big", result.Variant)
}

func TestIntEvaluationTypeMismatch(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"str-flag": {VariantKey: strPtr("v"), VariantValue: "not-an-int"},
		},
	}
	p := mustNewProvider(t, mock)
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
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.ObjectEvaluation(ctx, "obj-flag", nil, nil)
	assert.Equal(t, obj, result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
	assert.Equal(t, "config", result.Variant)
}

func TestContextPassedThrough(t *testing.T) {
	// Use a custom mock that captures the flag context
	var capturedContext flags.FlagContext
	captureMock := &contextCaptureMock{
		ready:           true,
		capturedContext: &capturedContext,
		variant:         flags.SelectedVariant{VariantKey: strPtr("v"), VariantValue: true},
	}
	p := mustNewProvider(t, captureMock)
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

func TestContextUnwrapsValues(t *testing.T) {
	var capturedContext flags.FlagContext
	captureMock := &contextCaptureMock{
		ready:           true,
		capturedContext: &capturedContext,
		variant:         flags.SelectedVariant{VariantKey: strPtr("v"), VariantValue: true},
	}
	p := mustNewProvider(t, captureMock)
	ctx := context.Background()

	evalCtx := of.FlattenedContext{
		"distinct_id":     "user123",
		"whole_float":     float64(42),
		"fractional":      3.14,
		"nested_map":      map[string]any{"inner_float": float64(10)},
		"nested_slice":    []any{float64(1), float64(2), "three"},
	}

	p.BooleanEvaluation(ctx, "flag", false, evalCtx)

	assert.Equal(t, "user123", capturedContext["distinct_id"])
	assert.Equal(t, int64(42), capturedContext["whole_float"])
	assert.Equal(t, 3.14, capturedContext["fractional"])
	assert.Equal(t, map[string]any{"inner_float": int64(10)}, capturedContext["nested_map"])
	assert.Equal(t, []any{int64(1), int64(2), "three"}, capturedContext["nested_slice"])
}

func TestDefaultValueReturnedOnError(t *testing.T) {
	mock := &mockFlagsProvider{
		ready:    false,
		variants: map[string]flags.SelectedVariant{},
	}
	p := mustNewProvider(t, mock)
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
	p := mustNewProvider(t, captureMock)
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
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "remote-flag", "default", nil)
	assert.Equal(t, "remote-value", result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
}

func TestShutdownCallsStopPolling(t *testing.T) {
	mock := &mockShutdownProvider{}
	p := mustNewProvider(t, mock)
	p.Shutdown()
	assert.True(t, mock.stopped)
}

func TestShutdownNoOpForRemote(t *testing.T) {
	p := mustNewProvider(t, &mockRemoteFlagsProvider{})
	// Should not panic when provider has no StopPollingForDefinitions
	p.Shutdown()
}

// mockShutdownProvider implements FlagsProvider + StopPollingForDefinitions.
type mockShutdownProvider struct {
	mockFlagsProvider
	stopped bool
}

func (m *mockShutdownProvider) StopPollingForDefinitions() {
	m.stopped = true
}

// contextCaptureMock captures the FlagContext passed to GetVariant.
type contextCaptureMock struct {
	ready           bool
	capturedContext *flags.FlagContext
	variant         flags.SelectedVariant
}

func (m *contextCaptureMock) GetVariant(_ context.Context, _ string, _ flags.SelectedVariant, flagContext flags.FlagContext, _ bool) (flags.SelectedVariant, error) {
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

func (m *errorFlagsProvider) GetVariant(_ context.Context, _ string, fallbackVariant flags.SelectedVariant, _ flags.FlagContext, _ bool) (flags.SelectedVariant, error) {
	return fallbackVariant, fmt.Errorf("sdk error")
}

func (m *errorFlagsProvider) AreFlagsReady() bool {
	return m.ready
}

func TestSDKErrorReturnsDefault(t *testing.T) {
	mock := &errorFlagsProvider{ready: true}
	p := mustNewProvider(t, mock)
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
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.FloatEvaluation(ctx, "int-as-float", 0.0, nil)
	assert.Equal(t, 42.0, result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
}

func TestIntEvaluationFromNativeInt64(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"int64-flag": {VariantKey: strPtr("limit"), VariantValue: int64(100)},
		},
	}
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "int64-flag", 0, nil)
	assert.Equal(t, int64(100), result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
}

func TestIntEvaluationNonWholeFloat(t *testing.T) {
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"pi": {VariantKey: strPtr("v"), VariantValue: 3.14},
		},
	}
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.IntEvaluation(ctx, "pi", 0, nil)
	assert.Equal(t, int64(0), result.Value)
	assert.Contains(t, result.ResolutionError.Error(), "TYPE_MISMATCH")
}

func TestNullVariantKeyReturnsFlagNotFound(t *testing.T) {
	// When VariantKey is nil, the provider should treat it as flag not found.
	mock := &mockFlagsProvider{
		ready: true,
		variants: map[string]flags.SelectedVariant{
			"nil-variant-flag": {VariantKey: nil, VariantValue: "some-value"},
		},
	}
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	boolResult := p.BooleanEvaluation(ctx, "nil-variant-flag", false, nil)
	assert.Equal(t, false, boolResult.Value)
	assert.Equal(t, of.DefaultReason, boolResult.Reason)
	assert.Contains(t, boolResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	strResult := p.StringEvaluation(ctx, "nil-variant-flag", "default", nil)
	assert.Equal(t, "default", strResult.Value)
	assert.Equal(t, of.DefaultReason, strResult.Reason)
	assert.Contains(t, strResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	floatResult := p.FloatEvaluation(ctx, "nil-variant-flag", 1.0, nil)
	assert.Equal(t, 1.0, floatResult.Value)
	assert.Equal(t, of.DefaultReason, floatResult.Reason)
	assert.Contains(t, floatResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	intResult := p.IntEvaluation(ctx, "nil-variant-flag", 5, nil)
	assert.Equal(t, int64(5), intResult.Value)
	assert.Equal(t, of.DefaultReason, intResult.Reason)
	assert.Contains(t, intResult.ResolutionError.Error(), "FLAG_NOT_FOUND")

	objResult := p.ObjectEvaluation(ctx, "nil-variant-flag", nil, nil)
	assert.Nil(t, objResult.Value)
	assert.Equal(t, of.DefaultReason, objResult.Reason)
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
	p := mustNewProvider(t, mock)
	ctx := context.Background()

	result := p.StringEvaluation(ctx, "empty-key-flag", "default", nil)
	assert.Equal(t, "value", result.Value)
	assert.Equal(t, of.TargetingMatchReason, result.Reason)
	assert.Equal(t, "", result.Variant)
}

func TestFlagNotFoundAllTypes(t *testing.T) {
	mock := &mockFlagsProvider{ready: true, variants: map[string]flags.SelectedVariant{}}
	p := mustNewProvider(t, mock)
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
