package openfeature

import (
	"context"
	"errors"
	"fmt"

	mixpanel "github.com/mixpanel/mixpanel-go/v2"
	"github.com/mixpanel/mixpanel-go/v2/flags"
	of "github.com/open-feature/go-sdk/openfeature"
)

// FlagsProvider is the interface satisfied by both LocalFeatureFlagsProvider and RemoteFeatureFlagsProvider.
type FlagsProvider interface {
	GetVariant(ctx context.Context, flagKey string, fallbackVariant flags.SelectedVariant, flagContext flags.FlagContext, reportExposure bool) (flags.SelectedVariant, error)
}

// Provider implements the OpenFeature FeatureProvider interface for Mixpanel.
// It wraps either a LocalFeatureFlagsProvider or RemoteFeatureFlagsProvider,
// delegating flag evaluation to the Mixpanel SDK while conforming to the
// OpenFeature provider contract.
type Provider struct {
	of.NoopStateHandler
	of.NoopEventHandler
	flagsProvider FlagsProvider

	// Mixpanel is the underlying ApiClient instance, available when the provider
	// was created via NewProviderWithLocalConfig or NewProviderWithRemoteConfig.
	Mixpanel *mixpanel.ApiClient
}

// Compile-time check that Provider satisfies the FeatureProvider interface.
var _ of.FeatureProvider = (*Provider)(nil)

// NewProvider creates a new Mixpanel OpenFeature provider wrapping the given flags provider.
func NewProvider(flagsProvider FlagsProvider) (*Provider, error) {
	if flagsProvider == nil {
		return nil, errors.New("flagsProvider must not be nil")
	}
	return &Provider{flagsProvider: flagsProvider}, nil
}

// NewProviderWithLocalConfig creates a new Mixpanel OpenFeature provider using local flag evaluation.
// It creates an ApiClient with a LocalFeatureFlagsProvider, starts polling for definitions, and returns the provider.
// The ApiClient is accessible via the Mixpanel field.
func NewProviderWithLocalConfig(token string, config flags.LocalFlagsConfig) (*Provider, error) {
	mp := mixpanel.NewApiClient(token, mixpanel.WithLocalFlags(config))
	if err := mp.LocalFlags.StartPollingForDefinitions(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start polling for definitions: %w", err)
	}
	provider, _ := NewProvider(mp.LocalFlags)
	provider.Mixpanel = mp
	return provider, nil
}

// NewProviderWithRemoteConfig creates a new Mixpanel OpenFeature provider using remote flag evaluation.
// It creates an ApiClient with a RemoteFeatureFlagsProvider and returns the provider.
// The ApiClient is accessible via the Mixpanel field.
func NewProviderWithRemoteConfig(token string, config flags.RemoteFlagsConfig) (*Provider, error) {
	mp := mixpanel.NewApiClient(token, mixpanel.WithRemoteFlags(config, nil))
	provider, _ := NewProvider(mp.RemoteFlags)
	provider.Mixpanel = mp
	return provider, nil
}

func (p *Provider) Metadata() of.Metadata {
	return of.Metadata{Name: "mixpanel-provider"}
}

func (p *Provider) Hooks() []of.Hook {
	return nil
}

func (p *Provider) Shutdown() {
	if shutdowner, ok := p.flagsProvider.(interface{ StopPollingForDefinitions() }); ok {
		shutdowner.StopPollingForDefinitions()
	}
}

func (p *Provider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, evalCtx of.FlattenedContext) of.BoolResolutionDetail {
	value, variant, err := p.resolve(ctx, flag, defaultValue, evalCtx)
	if err != nil {
		return of.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: errorDetail(err),
		}
	}

	boolVal, ok := value.(bool)
	if !ok {
		return of.BoolResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: typeMismatchDetail(value, "bool"),
		}
	}

	return of.BoolResolutionDetail{
		Value:                    boolVal,
		ProviderResolutionDetail: successDetail(variant),
	}
}

func (p *Provider) StringEvaluation(ctx context.Context, flag string, defaultValue string, evalCtx of.FlattenedContext) of.StringResolutionDetail {
	value, variant, err := p.resolve(ctx, flag, defaultValue, evalCtx)
	if err != nil {
		return of.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: errorDetail(err),
		}
	}

	strVal, ok := value.(string)
	if !ok {
		return of.StringResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: typeMismatchDetail(value, "string"),
		}
	}

	return of.StringResolutionDetail{
		Value:                    strVal,
		ProviderResolutionDetail: successDetail(variant),
	}
}

func (p *Provider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, evalCtx of.FlattenedContext) of.FloatResolutionDetail {
	value, variant, err := p.resolve(ctx, flag, defaultValue, evalCtx)
	if err != nil {
		return of.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: errorDetail(err),
		}
	}

	floatVal, ok := toFloat64(value)
	if !ok {
		return of.FloatResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: typeMismatchDetail(value, "float64"),
		}
	}

	return of.FloatResolutionDetail{
		Value:                    floatVal,
		ProviderResolutionDetail: successDetail(variant),
	}
}

func (p *Provider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, evalCtx of.FlattenedContext) of.IntResolutionDetail {
	value, variant, err := p.resolve(ctx, flag, defaultValue, evalCtx)
	if err != nil {
		return of.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: errorDetail(err),
		}
	}

	intVal, ok := toInt64(value)
	if !ok {
		return of.IntResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: typeMismatchDetail(value, "int64"),
		}
	}

	return of.IntResolutionDetail{
		Value:                    intVal,
		ProviderResolutionDetail: successDetail(variant),
	}
}

func (p *Provider) ObjectEvaluation(ctx context.Context, flag string, defaultValue any, evalCtx of.FlattenedContext) of.InterfaceResolutionDetail {
	value, variant, err := p.resolve(ctx, flag, defaultValue, evalCtx)
	if err != nil {
		return of.InterfaceResolutionDetail{
			Value:                    defaultValue,
			ProviderResolutionDetail: errorDetail(err),
		}
	}

	// encoding/json decodes all JSON numbers as float64, so nested whole
	// numbers in object-typed flag values surface as float64 — a caller
	// writing config["count"].(int) panics. Recurse the same float64 → int64
	// coercion already used on the input side (see unwrapValue / toFlagContext).
	return of.InterfaceResolutionDetail{
		Value:                    unwrapValue(value),
		ProviderResolutionDetail: successDetail(variant),
	}
}

type resolveErrorKind int

const (
	errProviderNotReady resolveErrorKind = iota
	errFlagNotFound
	errTargetingKeyMissing
	errNoRolloutMatch
	errGeneral
)

type resolveError struct {
	kind    resolveErrorKind
	message string
}

func (e *resolveError) Error() string { return e.message }

// resolve calls the underlying flags provider and returns the value, variant key, and any error.
func (p *Provider) resolve(ctx context.Context, flagKey string, defaultValue any, evalCtx of.FlattenedContext) (any, string, error) {
	if checker, ok := p.flagsProvider.(interface{ AreFlagsReady() bool }); ok && !checker.AreFlagsReady() {
		return nil, "", &resolveError{kind: errProviderNotReady, message: "provider not ready"}
	}

	flagContext := toFlagContext(evalCtx)

	fallback := flags.SelectedVariant{VariantValue: defaultValue}
	variant, err := p.flagsProvider.GetVariant(ctx, flagKey, fallback, flagContext, true)
	if err != nil {
		return nil, "", &resolveError{kind: errGeneral, message: err.Error()}
	}

	// A fallback source means the SDK had no real variant to serve. The
	// reason discriminates why (flag missing, context key missing, no rule
	// matched, backend error) so we map each to the OpenFeature error the
	// spec assigns to it instead of collapsing every fallback to FLAG_NOT_FOUND.
	if variant.VariantSource == flags.VariantSourceFallback {
		return nil, "", fallbackReasonError(variant.FallbackReason, flagKey)
	}

	// Belt-and-suspenders: if a custom flags provider returns a nil
	// VariantKey without tagging the variant as a fallback, treat it as
	// flag-not-found rather than a successful match on nil.
	if variant.VariantKey == nil {
		return nil, "", &resolveError{kind: errFlagNotFound, message: fmt.Sprintf("flag not found: %s", flagKey)}
	}

	return variant.VariantValue, *variant.VariantKey, nil
}

// fallbackReasonError maps a base-SDK FallbackReason constant to the resolve
// error kind that errorDetail translates into the correct OpenFeature response.
// The mapping matches PHP/Python/Ruby: NO_ROLLOUT_MATCH is not an error
// (returns DEFAULT reason with the caller's default value), everything else
// carries an error code.
func fallbackReasonError(reason, flagKey string) error {
	switch reason {
	case flags.FallbackReasonFlagNotFound:
		return &resolveError{kind: errFlagNotFound, message: fmt.Sprintf("flag not found: %s", flagKey)}
	case flags.FallbackReasonMissingContextKey:
		return &resolveError{kind: errTargetingKeyMissing, message: fmt.Sprintf("missing targeting key for flag: %s", flagKey)}
	case flags.FallbackReasonNoRolloutMatch:
		return &resolveError{kind: errNoRolloutMatch, message: ""}
	case flags.FallbackReasonBackendError:
		return &resolveError{kind: errGeneral, message: fmt.Sprintf("backend error evaluating flag: %s", flagKey)}
	default:
		// Unknown reason (e.g., new value added to base SDK without a
		// corresponding case here) — surface as general error so silent
		// regressions don't get misread as successful evaluations.
		return &resolveError{kind: errGeneral, message: fmt.Sprintf("unrecognized fallback reason %q for flag: %s", reason, flagKey)}
	}
}

func toFlagContext(evalCtx of.FlattenedContext) flags.FlagContext {
	fc := make(flags.FlagContext, len(evalCtx))
	for k, v := range evalCtx {
		fc[k] = unwrapValue(v)
	}
	return fc
}

func unwrapValue(v any) any {
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = unwrapValue(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			result[k] = unwrapValue(item)
		}
		return result
	default:
		return val
	}
}

func successDetail(variant string) of.ProviderResolutionDetail {
	return of.ProviderResolutionDetail{
		Reason:  of.TargetingMatchReason,
		Variant: variant,
	}
}

func errorDetail(err error) of.ProviderResolutionDetail {
	re, ok := err.(*resolveError)
	if !ok {
		return of.ProviderResolutionDetail{
			ResolutionError: of.NewGeneralResolutionError(err.Error()),
			Reason:          of.ErrorReason,
		}
	}

	var resErr of.ResolutionError
	var reason of.Reason
	switch re.kind {
	case errProviderNotReady:
		resErr = of.NewProviderNotReadyResolutionError(re.message)
		reason = of.ErrorReason
	case errFlagNotFound:
		resErr = of.NewFlagNotFoundResolutionError(re.message)
		reason = of.DefaultReason
	case errTargetingKeyMissing:
		resErr = of.NewTargetingKeyMissingResolutionError(re.message)
		reason = of.ErrorReason
	case errNoRolloutMatch:
		// Flag exists but no rule matched — per the OpenFeature spec this
		// is DEFAULT with no error, distinct from FLAG_NOT_FOUND.
		return of.ProviderResolutionDetail{Reason: of.DefaultReason}
	default:
		resErr = of.NewGeneralResolutionError(re.message)
		reason = of.ErrorReason
	}

	return of.ProviderResolutionDetail{
		ResolutionError: resErr,
		Reason:          reason,
	}
}

func typeMismatchDetail(value any, expected string) of.ProviderResolutionDetail {
	return of.ProviderResolutionDetail{
		ResolutionError: of.NewTypeMismatchResolutionError(fmt.Sprintf("expected %s, got %T", expected, value)),
		Reason:          of.ErrorReason,
	}
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case float64:
		if n == float64(int64(n)) {
			return int64(n), true
		}
		return 0, false
	case float32:
		if n == float32(int64(n)) {
			return int64(n), true
		}
		return 0, false
	default:
		return 0, false
	}
}
