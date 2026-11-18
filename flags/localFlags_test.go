package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestNormalizedHash(t *testing.T) {
	t.Run("hash consistency with other SDKs", func(t *testing.T) {
		hash1 := normalizedHash("user-123", "test-salt")
		require.GreaterOrEqual(t, hash1, 0.0)
		require.Less(t, hash1, 1.0)

		hash2 := normalizedHash("user-123", "test-salt")
		require.Equal(t, hash1, hash2)

		hash3 := normalizedHash("user-456", "test-salt")
		require.NotEqual(t, hash1, hash3)
	})

	t.Run("matches known test vectors", func(t *testing.T) {
		hash1 := normalizedHash("abc", "variant")
		require.Equal(t, 0.72, hash1)

		hash2 := normalizedHash("def", "variant")
		require.Equal(t, 0.21, hash2)
	})

	t.Run("hash distribution", func(t *testing.T) {
		var below50 int
		for i := 0; i < 1000; i++ {
			hash := normalizedHash(string(rune(i)), "salt")
			if hash < 0.5 {
				below50++
			}
		}
		require.Greater(t, below50, 300)
		require.Less(t, below50, 700)
	})
}

func TestLocalFeatureFlagsProvider_AreFlagsReady(t *testing.T) {
	t.Run("returns false before polling and true after", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		require.False(t, provider.AreFlagsReady())

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, experimentationFlagsResponse{Flags: []ExperimentationFlag{}}))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		require.True(t, provider.AreFlagsReady())
	})
}

func TestLocalFeatureFlagsProvider_GetVariantValue(t *testing.T) {
	t.Run("returns fallback when flag not found", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, experimentationFlagsResponse{Flags: []ExperimentationFlag{}}))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		result, err := provider.GetVariantValue(ctx, "nonexistent", "fallback", FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.Equal(t, "fallback", result)
	})

	t.Run("returns variant value when flag found", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Name:    "Test Flag",
					Key:     "test-flag",
					Status:  "active",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{
							{Key: "control", Value: false, Split: 0.5},
							{Key: "variant", Value: true, Split: 0.5},
						},
						Rollout: []Rollout{
							{RolloutPercentage: 1.0},
						},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		result, err := provider.GetVariantValue(ctx, "test-flag", "fallback", FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.NotEqual(t, "fallback", result)
	})
}

func TestLocalFeatureFlagsProvider_IsEnabled(t *testing.T) {
	t.Run("returns true when variant value is true", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Name:    "Boolean Flag",
					Key:     "bool-flag",
					Status:  "active",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{
							{Key: "enabled", Value: true, Split: 1.0},
						},
						Rollout: []Rollout{
							{RolloutPercentage: 1.0},
						},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		result, err := provider.IsEnabled(ctx, "bool-flag", FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("returns false when flag not found", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, experimentationFlagsResponse{Flags: []ExperimentationFlag{}}))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		result, err := provider.IsEnabled(ctx, "nonexistent", FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.False(t, result)
	})
}

func TestLocalFeatureFlagsProvider_TestUserOverride(t *testing.T) {
	t.Run("returns override variant for test user", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		var trackedEvents []map[string]any
		tracker := func(distinctID string, eventName string, props map[string]any) {
			trackedEvents = append(trackedEvents, props)
		}

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, tracker)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Name:    "Test Flag",
					Key:     "test-flag",
					Status:  "active",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{
							{Key: "control", Value: "control-value", Split: 0.5},
							{Key: "test", Value: "test-value", Split: 0.5},
						},
						Rollout: []Rollout{
							{RolloutPercentage: 1.0},
						},
						Test: &FlagTestUsers{
							Users: map[string]string{
								"qa-user": "test",
							},
						},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		result, err := provider.GetVariantValue(ctx, "test-flag", "fallback", FlagContext{"distinct_id": "qa-user"})
		require.NoError(t, err)
		require.Equal(t, "test-value", result)

		require.Len(t, trackedEvents, 1)
		require.Equal(t, true, trackedEvents[0]["$is_qa_tester"])
	})
}

func TestLocalFeatureFlagsProvider_RuntimeEvaluation(t *testing.T) {
	t.Run("evaluates JSON Logic rule", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Name:    "JSON Logic Flag",
					Key:     "jsonlogic-flag",
					Status:  "active",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{
							{Key: "variant", Value: "enabled", Split: 1.0},
						},
						Rollout: []Rollout{
							{
								RolloutPercentage: 1.0,
								RuntimeEvaluationRule: map[string]any{
									"==": []any{map[string]any{"var": "plan"}, "premium"},
								},
							},
						},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		result, err := provider.GetVariantValue(ctx, "jsonlogic-flag", "fallback", FlagContext{
			"distinct_id":       "user1",
			"custom_properties": map[string]any{"plan": "premium"},
		})
		require.NoError(t, err)
		require.Equal(t, "enabled", result)

		result, err = provider.GetVariantValue(ctx, "jsonlogic-flag", "fallback", FlagContext{
			"distinct_id":       "user2",
			"custom_properties": map[string]any{"plan": "free"},
		})
		require.NoError(t, err)
		require.Equal(t, "fallback", result)
	})
}

func TestLocalFeatureFlagsProvider_CustomOperators(t *testing.T) {
	newProvider := func(t *testing.T, rule map[string]any) *LocalFeatureFlagsProvider {
		t.Helper()
		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false
		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Key:     "custom-op-flag",
					Status:  "active",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{{Key: "variant", Value: "enabled", Split: 1.0}},
						Rollout:  []Rollout{{RolloutPercentage: 1.0, RuntimeEvaluationRule: rule}},
					},
				},
			},
		}
		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))
		require.NoError(t, provider.StartPollingForDefinitions(context.Background()))
		return provider
	}

	// 2026-07-16T00:00:00Z, as epoch milliseconds — the shape the feature-flags UI emits.
	const jul16Ms = 1_784_160_000_000

	t.Run("semver_compare routes through the provider", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		rule := map[string]any{"semver_compare": []any{map[string]any{"var": "app_version"}, ">=", "2.0.0"}}
		provider := newProvider(t, rule)

		result, err := provider.GetVariantValue(context.Background(), "custom-op-flag", "fallback", FlagContext{
			"distinct_id":       "user1",
			"custom_properties": map[string]any{"app_version": "2.3.0"},
		})
		require.NoError(t, err)
		require.Equal(t, "enabled", result)

		result, err = provider.GetVariantValue(context.Background(), "custom-op-flag", "fallback", FlagContext{
			"distinct_id":       "user2",
			"custom_properties": map[string]any{"app_version": "1.9.0"},
		})
		require.NoError(t, err)
		require.Equal(t, "fallback", result)
	})

	t.Run("datetime_compare routes through the provider", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		rule := map[string]any{"datetime_compare": []any{map[string]any{"var": "signup"}, "<", float64(jul16Ms)}}
		provider := newProvider(t, rule)

		result, err := provider.GetVariantValue(context.Background(), "custom-op-flag", "fallback", FlagContext{
			"distinct_id":       "user1",
			"custom_properties": map[string]any{"signup": "2026-07-15T00:00:00Z"},
		})
		require.NoError(t, err)
		require.Equal(t, "enabled", result)

		result, err = provider.GetVariantValue(context.Background(), "custom-op-flag", "fallback", FlagContext{
			"distinct_id":       "user2",
			"custom_properties": map[string]any{"signup": "2026-07-17T00:00:00Z"},
		})
		require.NoError(t, err)
		require.Equal(t, "fallback", result)
	})
}

func TestLocalFeatureFlagsProvider_GetAllVariants(t *testing.T) {
	t.Run("returns all applicable variants", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Key:     "flag-1",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{{Key: "v1", Value: "value1", Split: 1.0}},
						Rollout:  []Rollout{{RolloutPercentage: 1.0}},
					},
				},
				{
					ID:      "flag-2",
					Key:     "flag-2",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{{Key: "v2", Value: "value2", Split: 1.0}},
						Rollout:  []Rollout{{RolloutPercentage: 1.0}},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		variants, err := provider.GetAllVariants(ctx, FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.Len(t, variants, 2)
		require.Contains(t, variants, "flag-1")
		require.Contains(t, variants, "flag-2")
	})
}

func TestLocalFeatureFlagsProvider_ExposureTracking(t *testing.T) {
	t.Run("tracks exposure event with correct properties", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		var trackedDistinctID string
		var trackedEventName string
		var trackedProps map[string]any

		tracker := func(distinctID string, eventName string, props map[string]any) {
			trackedDistinctID = distinctID
			trackedEventName = eventName
			trackedProps = props
		}

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, tracker)

		experimentID := "exp-123"
		isActive := true

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:                 "flag-1",
					Key:                "test-flag",
					Context:            "distinct_id",
					ExperimentID:       &experimentID,
					IsExperimentActive: &isActive,
					Ruleset: RuleSet{
						Variants: []Variant{{Key: "variant", Value: "test", Split: 1.0}},
						Rollout:  []Rollout{{RolloutPercentage: 1.0}},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		_, err = provider.GetVariantValue(ctx, "test-flag", "fallback", FlagContext{"distinct_id": "user123"})
		require.NoError(t, err)

		require.Equal(t, "user123", trackedDistinctID)
		require.Equal(t, "$experiment_started", trackedEventName)
		require.Equal(t, "test-flag", trackedProps["Experiment name"])
		require.Equal(t, "feature_flag", trackedProps["$experiment_type"])
		require.Equal(t, "local", trackedProps["Flag evaluation mode"])
		require.Equal(t, "exp-123", trackedProps["$experiment_id"])
		require.Equal(t, true, trackedProps["$is_experiment_active"])
	})

	t.Run("does not track exposure when reportExposure is false", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		trackCount := 0
		tracker := func(distinctID string, eventName string, props map[string]any) {
			trackCount++
		}

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, tracker)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Key:     "test-flag",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{{Key: "variant", Value: "test", Split: 1.0}},
						Rollout:  []Rollout{{RolloutPercentage: 1.0}},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		_, err = provider.GetVariant(ctx, "test-flag", SelectedVariant{}, FlagContext{"distinct_id": "user123"}, false)
		require.NoError(t, err)
		require.Equal(t, 0, trackCount)
	})
}

func TestLocalFeatureFlagsProvider_VariantSplits(t *testing.T) {
	t.Run("respects variant splits from rollout", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{
				{
					ID:      "flag-1",
					Key:     "split-flag",
					Context: "distinct_id",
					Ruleset: RuleSet{
						Variants: []Variant{
							{Key: "control", Value: "control", Split: 0.5},
							{Key: "variant", Value: "variant", Split: 0.5},
						},
						Rollout: []Rollout{
							{
								RolloutPercentage: 1.0,
								VariantSplits: map[string]float64{
									"control": 0.9,
									"variant": 0.1,
								},
							},
						},
					},
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		err := provider.StartPollingForDefinitions(ctx)
		require.NoError(t, err)

		controlCount := 0
		for i := 0; i < 100; i++ {
			result, err := provider.GetVariant(ctx, "split-flag", SelectedVariant{}, FlagContext{
				"distinct_id": json.Number(string(rune(i))),
			}, false)
			require.NoError(t, err)
			if result.VariantValue == "control" {
				controlCount++
			}
		}

		require.Greater(t, controlCount, 50)
	})
}

// SDK-79: every fallback path on get_variant must be tagged distinctly via
// VariantSource so the OpenFeature wrapper can map it correctly.
func TestLocalFeatureFlagsProvider_GetVariant_VariantSourceTagging(t *testing.T) {
	makeFlag := func(rolloutPct float64, ctxKey string) ExperimentationFlag {
		return ExperimentationFlag{
			Key:     "test-flag",
			Context: ctxKey,
			Ruleset: RuleSet{
				Variants: []Variant{
					{Key: "control", Value: "control", IsControl: true, Split: 100.0},
				},
				Rollout: []Rollout{{RolloutPercentage: rolloutPct}},
			},
		}
	}

	setup := func(t *testing.T, defs []ExperimentationFlag) *LocalFeatureFlagsProvider {
		t.Helper()
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)

		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false
		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, nil)

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, experimentationFlagsResponse{Flags: defs}))
		require.NoError(t, provider.StartPollingForDefinitions(context.Background())) //nolint:contextcheck

		return provider
	}

	t.Run("tags matched variants as local with no fallback_reason", func(t *testing.T) {
		provider := setup(t, []ExperimentationFlag{makeFlag(100.0, "distinct_id")})
		result, err := provider.GetVariant(context.Background(), "test-flag",
			SelectedVariant{VariantValue: "fb"}, FlagContext{"distinct_id": "u1"}, false)
		require.NoError(t, err)
		require.Equal(t, VariantSourceLocal, result.VariantSource)
		require.Empty(t, result.FallbackReason)
		require.NotNil(t, result.VariantKey)
	})

	t.Run("tags missing flag as fallback / FLAG_NOT_FOUND", func(t *testing.T) {
		provider := setup(t, nil)
		result, err := provider.GetVariant(context.Background(), "missing",
			SelectedVariant{VariantValue: "fb"}, FlagContext{"distinct_id": "u1"}, false)
		require.NoError(t, err)
		require.Equal(t, VariantSourceFallback, result.VariantSource)
		require.Equal(t, FallbackReasonFlagNotFound, result.FallbackReason)
		require.Equal(t, "fb", result.VariantValue)
	})

	t.Run("tags missing context as fallback / MISSING_CONTEXT_KEY", func(t *testing.T) {
		provider := setup(t, []ExperimentationFlag{makeFlag(100.0, "distinct_id")})
		result, err := provider.GetVariant(context.Background(), "test-flag",
			SelectedVariant{VariantValue: "fb"}, FlagContext{}, false)
		require.NoError(t, err)
		require.Equal(t, VariantSourceFallback, result.VariantSource)
		require.Equal(t, FallbackReasonMissingContextKey, result.FallbackReason)
	})

	t.Run("tags no-rollout-match as fallback / NO_ROLLOUT_MATCH", func(t *testing.T) {
		provider := setup(t, []ExperimentationFlag{makeFlag(0.0, "distinct_id")})
		result, err := provider.GetVariant(context.Background(), "test-flag",
			SelectedVariant{VariantValue: "fb"}, FlagContext{"distinct_id": "u1"}, false)
		require.NoError(t, err)
		require.Equal(t, VariantSourceFallback, result.VariantSource)
		require.Equal(t, FallbackReasonNoRolloutMatch, result.FallbackReason)
	})

	t.Run("tags rollout-evaluation failure as fallback / BACKEND_ERROR", func(t *testing.T) {
		// An unknown jsonlogic operator makes the rule evaluation return
		// an error, which the provider surfaces as FallbackReasonBackendError.
		flagWithBadRule := ExperimentationFlag{
			ID:      "flag-1",
			Key:     "test-flag",
			Context: "distinct_id",
			Ruleset: RuleSet{
				Variants: []Variant{{Key: "v", Value: "x", Split: 1.0}},
				Rollout: []Rollout{
					{
						RolloutPercentage: 1.0,
						RuntimeEvaluationRule: map[string]any{
							"this-is-not-a-real-operator": []any{1, 2},
						},
					},
				},
			},
		}
		provider := setup(t, []ExperimentationFlag{flagWithBadRule})
		result, err := provider.GetVariant(context.Background(), "test-flag",
			SelectedVariant{VariantValue: "fb"},
			FlagContext{"distinct_id": "u1", "custom_properties": map[string]any{"k": "v"}},
			false)
		require.Error(t, err)
		require.Equal(t, VariantSourceFallback, result.VariantSource)
		require.Equal(t, FallbackReasonBackendError, result.FallbackReason)
		require.Equal(t, "fb", result.VariantValue)
	})
}

func TestLowercaseKeysAndValues(t *testing.T) {
	t.Run("lowercases string keys and values", func(t *testing.T) {
		input := map[string]any{
			"Name": "John",
			"PLAN": "PREMIUM",
		}
		result := lowercaseKeysAndValues(input).(map[string]any)
		require.Equal(t, "john", result["name"])
		require.Equal(t, "premium", result["plan"])
	})

	t.Run("handles nested maps", func(t *testing.T) {
		input := map[string]any{
			"User": map[string]any{
				"Name": "John",
			},
		}
		result := lowercaseKeysAndValues(input).(map[string]any)
		user := result["user"].(map[string]any)
		require.Equal(t, "john", user["name"])
	})
}

func TestLocalFeatureFlagsProvider_ExposureExecutor(t *testing.T) {
	t.Run("invokes ExposureExecutor instead of calling tracker inline", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var executorCalls int
		var capturedSend func()
		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false
		config.ExposureExecutor = func(send func()) {
			executorCalls++
			capturedSend = send
		}

		var trackerCalls int
		tracker := func(_ string, _ string, _ map[string]any) { trackerCalls++ }

		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, tracker)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{{
				ID: "flag-1", Key: "test-flag", Context: "distinct_id",
				Ruleset: RuleSet{
					Variants: []Variant{{Key: "variant", Value: "test", Split: 1.0}},
					Rollout:  []Rollout{{RolloutPercentage: 1.0}},
				},
			}},
		}
		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		require.NoError(t, provider.StartPollingForDefinitions(ctx))

		_, err := provider.GetVariant(ctx, "test-flag", SelectedVariant{}, FlagContext{"distinct_id": "user123"}, true)
		require.NoError(t, err)

		require.Equal(t, 1, executorCalls, "executor should be invoked once")
		require.Equal(t, 0, trackerCalls, "tracker should not be called inline when executor is set")
		require.NotNil(t, capturedSend)

		capturedSend()
		require.Equal(t, 1, trackerCalls, "running the captured send closure invokes the tracker")
	})

	t.Run("nil ExposureExecutor runs tracker inline (default)", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		var trackerCalls int
		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false
		require.Nil(t, config.ExposureExecutor)

		tracker := func(_ string, _ string, _ map[string]any) { trackerCalls++ }
		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, tracker)

		flags := experimentationFlagsResponse{
			Flags: []ExperimentationFlag{{
				ID: "flag-1", Key: "test-flag", Context: "distinct_id",
				Ruleset: RuleSet{
					Variants: []Variant{{Key: "variant", Value: "test", Split: 1.0}},
					Rollout:  []Rollout{{RolloutPercentage: 1.0}},
				},
			}},
		}
		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
			httpmock.NewJsonResponderOrPanic(200, flags))

		ctx := context.Background()
		require.NoError(t, provider.StartPollingForDefinitions(ctx))

		_, err := provider.GetVariant(ctx, "test-flag", SelectedVariant{}, FlagContext{"distinct_id": "user123"}, true)
		require.NoError(t, err)
		require.Equal(t, 1, trackerCalls)
	})

	t.Run("TrackExposureEvent (manual API) also honors ExposureExecutor", func(t *testing.T) {
		var capturedSend func()
		var executorCalls int
		config := DefaultLocalFlagsConfig()
		config.EnablePolling = false
		config.ExposureExecutor = func(send func()) {
			executorCalls++
			capturedSend = send
		}

		var trackerCalls int
		tracker := func(_ string, _ string, _ map[string]any) { trackerCalls++ }
		provider := NewLocalFeatureFlagsProvider("test-token", "test", config, tracker)

		variantKey := "treatment"
		provider.TrackExposureEvent(context.Background(), "manual-flag",
			SelectedVariant{VariantKey: &variantKey, VariantValue: "x"},
			FlagContext{"distinct_id": "user123"})

		require.Equal(t, 1, executorCalls)
		require.Equal(t, 0, trackerCalls)
		capturedSend()
		require.Equal(t, 1, trackerCalls)
	})
}

// Hierarchical groups

const hgTemplate = "$company:{$company_id}|$user:{$user_id}"

type trackedExposure struct {
	distinctID string
	props      map[string]any
}

// newDefinitionsProvider returns a started local provider serving the given raw flag definitions.
func newDefinitionsProvider(t *testing.T, hierarchicalGroups bool, definitions ...string) (*LocalFeatureFlagsProvider, *[]trackedExposure) {
	t.Helper()
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)
	httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags/definitions",
		httpmock.NewStringResponder(200, `{"flags":[`+strings.Join(definitions, ",")+`]}`))

	config := DefaultLocalFlagsConfig()
	config.EnablePolling = false
	config.HierarchicalGroups = hierarchicalGroups

	var exposures []trackedExposure
	tracker := func(distinctID string, eventName string, props map[string]any) {
		exposures = append(exposures, trackedExposure{distinctID: distinctID, props: props})
	}
	provider := NewLocalFeatureFlagsProvider("test-token", "test", config, tracker)
	require.NoError(t, provider.StartPollingForDefinitions(context.Background()))
	return provider, &exposures
}

// flagDefinition returns a definition with one fully rolled out variant. extra is spliced into
// the JSON object, e.g. `"context_value_template":"..."`.
func flagDefinition(key, contextProperty, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return `{"id":"id-` + key + `","key":"` + key + `","status":"active","project_id":1,"context":"` + contextProperty + `"` + extra + `,
		"ruleset":{"variants":[{"key":"variant-a","value":"value-a","split":1}],"rollout":[{"rollout_percentage":1}]}}`
}

func templateField(template string) string {
	return `"context_value_template":"` + template + `"`
}

// hgTestDefinition is the shared cross-SDK key test definition.
const hgTestDefinition = `{"id":"id-hg","key":"hg-test","status":"active","project_id":1,"context":"distinct_id",
	"context_value_template":"$company:{$company_id}|$user:{$user_id}",
	"hash_salt":"880dc3e584b4452fb769808ca18dc9a6",
	"ruleset":{"variants":[{"key":"On","value":true,"split":1},{"key":"Off","value":false,"split":0}],
		"rollout":[{"rollout_percentage":0.5,"variant_splits":{"Off":0,"On":1}}],
		"test":{"users":{}}}}`

// hgTestSalt is the rollout salt of hgTestDefinition's first rollout for a flag with the given key.
func hgTestSalt(key string) string {
	return key + "880dc3e584b4452fb769808ca18dc9a60"
}

// hgDefinition returns hgTestDefinition under another key, context property and template.
func hgDefinition(key, contextProperty, template string) string {
	definition := strings.Replace(hgTestDefinition, `"key":"hg-test"`, `"key":"`+key+`"`, 1)
	definition = strings.Replace(definition, `"context":"distinct_id"`, `"context":"`+contextProperty+`"`, 1)
	field := ""
	if template != "" {
		field = templateField(template) + ","
	}
	return strings.Replace(definition, templateField(hgTemplate)+",", field, 1)
}

// The templates the service sends for each context property.
var hgTemplates = map[string]string{
	"distinct_id": "$company:{$company_id}|$user:{$user_id}",
	"device_id":   "{device_id}",
	"$user_id":    "$company:{$company_id}|$user:{$user_id}",
	"$company_id": "$company:{$company_id}",
	"department":  "$company:{$company_id}|department:{department}",
}

func TestResolveAssignmentKey(t *testing.T) {
	hgContext := FlagContext{"distinct_id": "user_123", "device_id": "device_abc", "$user_id": "user_123",
		"$company_id": "company_123", "department": "finance"}
	nonHGContext := FlagContext{"distinct_id": "user_123", "device_id": "device_abc", "$user_id": "user_123",
		"department": "finance"}

	tests := []struct {
		name            string
		contextProperty string
		template        string
		flagContext     FlagContext
		expectedKey     any
		expectedOK      bool
	}{
		{"distinct_id template", "distinct_id", hgTemplates["distinct_id"], hgContext, "$company:company_123|$user:user_123", true},
		{"device_id template", "device_id", hgTemplates["device_id"], hgContext, "device_abc", true},
		{"$user_id template", "$user_id", hgTemplates["$user_id"], hgContext, "$company:company_123|$user:user_123", true},
		{"$company_id template", "$company_id", hgTemplates["$company_id"], hgContext, "$company:company_123", true},
		{"department template", "department", hgTemplates["department"], hgContext, "$company:company_123|department:finance", true},

		{"non-HG distinct_id uses raw", "distinct_id", hgTemplates["distinct_id"], nonHGContext, "user_123", true},
		{"non-HG device_id uses raw", "device_id", hgTemplates["device_id"], nonHGContext, "device_abc", true},
		{"non-HG $user_id uses raw", "$user_id", hgTemplates["$user_id"], nonHGContext, "user_123", true},
		{"non-HG $company_id is missing", "$company_id", hgTemplates["$company_id"], nonHGContext, nil, false},
		{"non-HG department uses raw", "department", hgTemplates["department"], nonHGContext, "finance", true},

		{"absent placeholder falls back to raw", "distinct_id", hgTemplates["distinct_id"],
			FlagContext{"distinct_id": "raw", "$company_id": "c1"}, "raw", true},
		{"nil placeholder falls back to raw", "distinct_id", hgTemplates["distinct_id"],
			FlagContext{"distinct_id": "raw", "$company_id": "c1", "$user_id": nil}, "raw", true},
		{"empty placeholder value is used", "distinct_id", hgTemplates["distinct_id"],
			FlagContext{"distinct_id": "raw", "$company_id": "", "$user_id": "u1"}, "$company:|$user:u1", true},
		{"no template uses raw", "distinct_id", "", hgContext, "user_123", true},
		{"no template keeps a present nil raw value", "distinct_id", "", FlagContext{"distinct_id": nil}, nil, true},
		{"raw also missing", "distinct_id", hgTemplates["distinct_id"], FlagContext{"$company_id": "c1"}, nil, false},
		{"no template and raw missing", "distinct_id", "", FlagContext{"$company_id": "c1", "$user_id": "u1"}, nil, false},
		{"literal text outside braces", "account", "org/{a}-{b}!",
			FlagContext{"account": "raw", "a": "x", "b": 2}, "org/x-2!", true},
		{"template without placeholders is literal", "account", "fixed", FlagContext{}, "fixed", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := &ExperimentationFlag{Context: tt.contextProperty, ContextValueTemplate: tt.template}
			key, ok := resolveAssignmentKey(flag, tt.flagContext)
			require.Equal(t, tt.expectedOK, ok)
			require.Equal(t, tt.expectedKey, key)
		})
	}
}

func TestLocalFeatureFlagsProvider_ContextValueTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("rollout hashes on the resolved key", func(t *testing.T) {
		for property, template := range hgTemplates {
			t.Run(property, func(t *testing.T) {
				provider, _ := newDefinitionsProvider(t, true, hgDefinition("flag", property, template))

				var on, off int
				for i := 0; i < 50; i++ {
					flagCtx := FlagContext{"distinct_id": "user_123", "device_id": fmt.Sprintf("device_%d", i), "$user_id": "user_123",
						"$company_id": fmt.Sprintf("company_%d", i), "department": "finance"}
					expectedKey, ok := resolveAssignmentKey(&ExperimentationFlag{Context: property, ContextValueTemplate: template}, flagCtx)
					require.True(t, ok)

					variant, err := provider.GetVariant(ctx, "flag", SelectedVariant{}, flagCtx, false)
					require.NoError(t, err)
					expectedIn := normalizedHash(expectedKey.(string), hgTestSalt("flag")) < 0.5
					require.Equal(t, expectedIn, variant.VariantKey != nil, "key %s", expectedKey)
					if expectedIn {
						on++
					} else {
						off++
					}
				}
				// A 50% rollout over 50 keys exercises both sides of the hash.
				require.Positive(t, on)
				require.Positive(t, off)
			})
		}
	})

	t.Run("non-HG context gets the same variant as without a template", func(t *testing.T) {
		templated, _ := newDefinitionsProvider(t, false, hgTestDefinition)
		var results []SelectedVariant
		for i := 0; i < 50; i++ {
			variant, err := templated.GetVariant(ctx, "hg-test", SelectedVariant{},
				FlagContext{"distinct_id": fmt.Sprintf("user_%d", i), "$user_id": fmt.Sprintf("user_%d", i)}, false)
			require.NoError(t, err)
			results = append(results, variant)
		}

		// The same definition without a template is how main reads it.
		plain, _ := newDefinitionsProvider(t, false, hgDefinition("hg-test", "distinct_id", ""))
		var on int
		for i := 0; i < 50; i++ {
			distinctID := fmt.Sprintf("user_%d", i)
			variant, err := plain.GetVariant(ctx, "hg-test", SelectedVariant{},
				FlagContext{"distinct_id": distinctID, "$user_id": distinctID}, false)
			require.NoError(t, err)
			require.Equal(t, variant, results[i], distinctID)
			require.Equal(t, normalizedHash(distinctID, hgTestSalt("hg-test")) < 0.5, variant.VariantKey != nil, distinctID)
			if variant.VariantKey != nil {
				on++
			}
		}
		require.Positive(t, on)
		require.Less(t, on, 50)
	})

	t.Run("raw value also missing returns the fallback", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", templateField(hgTemplates["distinct_id"])))

		variant, err := provider.GetVariant(ctx, "flag", SelectedVariant{}, FlagContext{"$company_id": "acme"}, true)
		require.NoError(t, err)
		require.Equal(t, FallbackReasonMissingContextKey, variant.FallbackReason)
		require.Empty(t, *exposures)
	})

	t.Run("template resolves without a raw value", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", templateField(hgTemplates["distinct_id"])))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)
		require.Equal(t, "value-a", result)
	})
}

func TestLocalFeatureFlagsProvider_ContextValueTemplateTestUsers(t *testing.T) {
	ctx := context.Background()
	definition := func(contextProperty, template string) string {
		extra := ""
		if template != "" {
			extra = "," + templateField(template)
		}
		return `{"id":"id","key":"flag","status":"active","project_id":1,"context":"` + contextProperty + `"` + extra + `,
			"ruleset":{"variants":[{"key":"control","value":"control-value","split":1},{"key":"treatment","value":"treatment-value","split":0}],
				"rollout":[{"rollout_percentage":1}],
				"test":{"users":{"$company:acme|$user:user-123":"treatment","raw-user":"treatment"}}}}`
	}

	tests := []struct {
		name            string
		contextProperty string
		template        string
		flagContext     FlagContext
		expected        string
	}{
		{"distinct_id flag looks up the resolved key", "distinct_id", hgTemplate,
			FlagContext{"distinct_id": "other", "$company_id": "acme", "$user_id": "user-123"}, "treatment-value"},
		{"distinct_id flag ignores the raw distinct id once the template resolves", "distinct_id", hgTemplate,
			FlagContext{"distinct_id": "raw-user", "$company_id": "acme", "$user_id": "other"}, "control-value"},
		{"distinct_id flag looks up the raw distinct id when the template does not resolve", "distinct_id", hgTemplate,
			FlagContext{"distinct_id": "raw-user"}, "treatment-value"},
		{"distinct_id flag without template looks up the raw distinct id", "distinct_id", "",
			FlagContext{"distinct_id": "raw-user", "$company_id": "acme", "$user_id": "user-123"}, "treatment-value"},
		{"other flag looks up the raw distinct id", "account", hgTemplate,
			FlagContext{"account": "a1", "distinct_id": "raw-user", "$company_id": "acme", "$user_id": "other"}, "treatment-value"},
		{"other flag does not look up the resolved key", "account", hgTemplate,
			FlagContext{"account": "a1", "$company_id": "acme", "$user_id": "user-123"}, "control-value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, exposures := newDefinitionsProvider(t, true, definition(tt.contextProperty, tt.template))

			result, err := provider.GetVariantValue(ctx, "flag", "fallback", tt.flagContext)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
			require.Len(t, *exposures, 1)
			require.Equal(t, tt.expected == "treatment-value", (*exposures)[0].props["$is_qa_tester"] == true)
		})
	}
}

func TestLocalFeatureFlagsProvider_HierarchicalExposure(t *testing.T) {
	ctx := context.Background()

	t.Run("carries user and company without distinct id", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", templateField(hgTemplate)))

		_, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)

		require.Len(t, *exposures, 1)
		exposure := (*exposures)[0]
		require.Equal(t, "", exposure.distinctID)
		require.Equal(t, "user-123", exposure.props["$user_id"])
		require.Equal(t, "acme", exposure.props["$company_id"])
		require.NotContains(t, exposure.props, "distinct_id")
		require.Equal(t, "local", exposure.props["Flag evaluation mode"])
	})

	t.Run("carries user, company and distinct id when sent", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", ""))

		_, err := provider.GetVariantValue(ctx, "flag", "fallback",
			FlagContext{"distinct_id": "$company:acme|$user:user-123", "$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)

		require.Len(t, *exposures, 1)
		require.Equal(t, "$company:acme|$user:user-123", (*exposures)[0].distinctID)
		require.Equal(t, "user-123", (*exposures)[0].props["$user_id"])
		require.Equal(t, "acme", (*exposures)[0].props["$company_id"])
	})

	t.Run("company alone attributes the exposure", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, true, flagDefinition("flag", "account", ""))

		_, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"account": "a1", "$company_id": "acme"})
		require.NoError(t, err)

		require.Len(t, *exposures, 1)
		require.Equal(t, "", (*exposures)[0].distinctID)
		require.Equal(t, "acme", (*exposures)[0].props["$company_id"])
		require.NotContains(t, (*exposures)[0].props, "$user_id")
	})

	t.Run("drops exposure without company or distinct id", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, true, flagDefinition("flag", "account", ""))

		for _, flagCtx := range []FlagContext{
			{"account": "a1", "$user_id": "user-123"},
			{"account": "a1", "$company_id": "", "distinct_id": ""},
		} {
			result, err := provider.GetVariantValue(ctx, "flag", "fallback", flagCtx)
			require.NoError(t, err)
			require.Equal(t, "value-a", result)
		}
		require.Empty(t, *exposures)
	})

	t.Run("legacy project ignores company and user in context", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, false, flagDefinition("flag", "distinct_id", ""))

		_, err := provider.GetVariantValue(ctx, "flag", "fallback",
			FlagContext{"distinct_id": "user-123", "$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)

		require.Len(t, *exposures, 1)
		require.Equal(t, "user-123", (*exposures)[0].distinctID)
		require.NotContains(t, (*exposures)[0].props, "$company_id")
		require.NotContains(t, (*exposures)[0].props, "$user_id")
	})

	t.Run("legacy project needs a distinct id", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, false, flagDefinition("flag", "account", ""))

		_, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"account": "a1", "$company_id": "acme", "$user_id": "u1"})
		require.NoError(t, err)
		require.Empty(t, *exposures)
	})
}

func TestLocalFeatureFlagsProvider_WithoutContextValueTemplateUnchanged(t *testing.T) {
	ctx := context.Background()
	provider, exposures := newDefinitionsProvider(t, false, hgTestDefinition, hgDefinition("plain", "distinct_id", ""))

	t.Run("hashes the distinct id exactly as before", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			distinctID := fmt.Sprintf("$company:acme|$user:u%d", i)
			variant, err := provider.GetVariant(ctx, "plain", SelectedVariant{}, FlagContext{"distinct_id": distinctID}, false)
			require.NoError(t, err)
			expectedIn := normalizedHash(distinctID, "plain880dc3e584b4452fb769808ca18dc9a60") < 0.5
			require.Equal(t, expectedIn, variant.VariantKey != nil, distinctID)
		}
	})

	t.Run("a present nil context value is still hashed", func(t *testing.T) {
		variant, err := provider.GetVariant(ctx, "plain", SelectedVariant{}, FlagContext{"distinct_id": nil}, false)
		require.NoError(t, err)
		require.NotEqual(t, FallbackReasonMissingContextKey, variant.FallbackReason)
	})

	t.Run("hierarchical context alone falls back", func(t *testing.T) {
		variant, err := provider.GetVariant(ctx, "plain", SelectedVariant{}, FlagContext{"$company_id": "acme", "$user_id": "u1"}, false)
		require.NoError(t, err)
		require.Equal(t, FallbackReasonMissingContextKey, variant.FallbackReason)
	})

	t.Run("exposure is the legacy event", func(t *testing.T) {
		*exposures = nil
		var userID string
		for i := 0; ; i++ {
			userID = fmt.Sprintf("u%d", i)
			if normalizedHash(userID, "plain880dc3e584b4452fb769808ca18dc9a60") < 0.5 {
				break
			}
		}
		_, err := provider.GetVariantValue(ctx, "plain", false, FlagContext{"distinct_id": userID})
		require.NoError(t, err)

		require.Len(t, *exposures, 1)
		require.Equal(t, userID, (*exposures)[0].distinctID)
		require.Equal(t, map[string]any{
			"Experiment name":            "plain",
			"Variant name":               (*exposures)[0].props["Variant name"],
			"$experiment_type":           "feature_flag",
			"Flag evaluation mode":       "local",
			"Variant fetch latency (ms)": (*exposures)[0].props["Variant fetch latency (ms)"],
		}, (*exposures)[0].props)
		require.Equal(t, "On", *(*exposures)[0].props["Variant name"].(*string))
	})
}
