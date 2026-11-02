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
// the JSON object, e.g. `"context_template":"..."`.
func flagDefinition(key, contextProperty, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return `{"id":"id-` + key + `","key":"` + key + `","status":"active","project_id":1,"context":"` + contextProperty + `"` + extra + `,
		"ruleset":{"variants":[{"key":"variant-a","value":"value-a","split":1}],"rollout":[{"rollout_percentage":1}]}}`
}

func templateField(template string) string {
	return `"context_template":"` + template + `"`
}

// hgTestDefinition is the shared cross-SDK key test definition.
const hgTestDefinition = `{"id":"id-hg","key":"hg-test","status":"active","project_id":1,"context":"distinct_id",
	"context_template":"$company:{$company_id}|$user:{$user_id}",
	"hash_salt":"880dc3e584b4452fb769808ca18dc9a6",
	"ruleset":{"variants":[{"key":"On","value":true,"split":1},{"key":"Off","value":false,"split":0}],
		"rollout":[{"rollout_percentage":0.5,"variant_splits":{"Off":0,"On":1}}],
		"test":{"users":{}}}}`

func TestLocalFeatureFlagsProvider_ContextTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("template context gets the same variant as the distinct id it builds", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, hgTestDefinition)

		var on, off int
		for i := 0; i < 50; i++ {
			userID := fmt.Sprintf("u%d", i)
			viaTemplate, err := provider.GetVariant(ctx, "hg-test", SelectedVariant{}, FlagContext{"$company_id": "acme", "$user_id": userID}, false)
			require.NoError(t, err)
			viaDistinctID, err := provider.GetVariant(ctx, "hg-test", SelectedVariant{}, FlagContext{"distinct_id": "$company:acme|$user:" + userID}, false)
			require.NoError(t, err)

			require.Equal(t, viaDistinctID, viaTemplate, "user %s", userID)
			if viaTemplate.VariantKey != nil {
				on++
			} else {
				off++
			}
		}
		// A 50% rollout over 50 users exercises both sides of the hash.
		require.Positive(t, on)
		require.Positive(t, off)
	})

	t.Run("template needs every placeholder", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, hgTestDefinition)

		for _, flagCtx := range []FlagContext{
			{"$user_id": "u1"},
			{"$company_id": "acme"},
			{"$company_id": "acme", "$user_id": ""},
			{"$company_id": nil, "$user_id": "u1"},
		} {
			variant, err := provider.GetVariant(ctx, "hg-test", SelectedVariant{}, flagCtx, false)
			require.NoError(t, err)
			require.Equal(t, FallbackReasonMissingContextKey, variant.FallbackReason, "%v", flagCtx)
		}
	})

	t.Run("template resolves the assignment key", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", templateField(hgTemplate)))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)
		require.Equal(t, "value-a", result)
	})

	t.Run("template is applied without hierarchical groups", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, false, flagDefinition("flag", "distinct_id", templateField(hgTemplate)))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)
		require.Equal(t, "value-a", result)
	})

	t.Run("context property is used as sent over the template", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, hgTestDefinition)

		for i := 0; i < 50; i++ {
			userID := fmt.Sprintf("u%d", i)
			direct, err := provider.GetVariant(ctx, "hg-test", SelectedVariant{}, FlagContext{"distinct_id": userID}, false)
			require.NoError(t, err)
			mixed, err := provider.GetVariant(ctx, "hg-test", SelectedVariant{},
				FlagContext{"distinct_id": userID, "$company_id": "acme", "$user_id": "other"}, false)
			require.NoError(t, err)
			require.Equal(t, direct, mixed, "user %s", userID)
		}
	})

	t.Run("hierarchical context without template or distinct id falls back", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", ""))

		variant, err := provider.GetVariant(ctx, "flag", SelectedVariant{}, FlagContext{"$company_id": "acme", "$user_id": "user-123"}, true)
		require.NoError(t, err)
		require.Equal(t, FallbackReasonMissingContextKey, variant.FallbackReason)
		require.Empty(t, *exposures)
	})

	t.Run("group and device flags hash on the raw context value", func(t *testing.T) {
		for _, property := range []string{"department", "device_id"} {
			provider, _ := newDefinitionsProvider(t, true, hgTestDefinition,
				strings.Replace(strings.Replace(hgTestDefinition, `"context":"distinct_id"`, `"context":"`+property+`"`, 1), `"key":"hg-test"`, `"key":"other"`, 1))

			for i := 0; i < 50; i++ {
				value := fmt.Sprintf("v%d", i)
				// Same key and salt is not possible across two flags, so compare against a
				// distinct_id flag hashing the same raw value under the same salt.
				onProperty, err := provider.GetVariant(ctx, "other", SelectedVariant{},
					FlagContext{property: value, "$company_id": "acme", "$user_id": "u1"}, false)
				require.NoError(t, err)
				expectedIn := normalizedHash(value, "other880dc3e584b4452fb769808ca18dc9a60") < 0.5
				require.Equal(t, expectedIn, onProperty.VariantKey != nil, "%s=%s", property, value)
			}
		}
	})

	t.Run("hierarchical project evaluates on distinct id alone", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", ""))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"distinct_id": "user-123"})
		require.NoError(t, err)
		require.Equal(t, "value-a", result)
	})
}

func TestLocalFeatureFlagsProvider_ContextTemplateDefinitions(t *testing.T) {
	ctx := context.Background()
	hgContext := FlagContext{"distinct_id": "user-123", "$company_id": "acme", "$user_id": "user-123"}

	for name, extra := range map[string]string{
		"newer template version":     `"context_template_version":99,` + templateField(hgTemplate),
		"newer version, no template": `"context_template_version":2`,
		"no placeholders":            templateField("$company:no-placeholders"),
		"empty placeholder":          templateField("$company:{}|$user:{$user_id}"),
		"blank placeholder":          templateField("$company:{  }|$user:{$user_id}"),
	} {
		t.Run("drops definition with "+name, func(t *testing.T) {
			provider, _ := newDefinitionsProvider(t, true,
				flagDefinition("good-flag", "distinct_id", ""),
				flagDefinition("flag", "distinct_id", extra))

			variant, err := provider.GetVariant(ctx, "flag", SelectedVariant{}, hgContext, false)
			require.NoError(t, err)
			require.Equal(t, FallbackReasonFlagNotFound, variant.FallbackReason)

			// One unusable definition must not take the rest of the payload with it.
			result, err := provider.GetVariantValue(ctx, "good-flag", "fallback", hgContext)
			require.NoError(t, err)
			require.Equal(t, "value-a", result)
		})
	}

	for name, extra := range map[string]string{
		"version 1":              `"context_template_version":1,` + templateField(hgTemplate),
		"absent version":         templateField(hgTemplate),
		"empty template":         templateField(""),
		"placeholder with space": templateField("{ $company_id }|{$user_id}"),
	} {
		t.Run("keeps definition with "+name, func(t *testing.T) {
			provider, _ := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", extra))

			result, err := provider.GetVariantValue(ctx, "flag", "fallback", hgContext)
			require.NoError(t, err)
			require.Equal(t, "value-a", result)
		})
	}

	t.Run("placeholder names are used verbatim", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, flagDefinition("flag", "distinct_id", templateField("{ $company_id }")))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"$company_id": "acme"})
		require.NoError(t, err)
		require.Equal(t, "fallback", result)

		result, err = provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{" $company_id ": "acme"})
		require.NoError(t, err)
		require.Equal(t, "value-a", result)
	})
}

func TestLocalFeatureFlagsProvider_ContextTemplateTestUsers(t *testing.T) {
	ctx := context.Background()
	definition := func(extra string) string {
		if extra != "" {
			extra = "," + extra
		}
		return `{"id":"id","key":"flag","status":"active","project_id":1,"context":"distinct_id"` + extra + `,
			"ruleset":{"variants":[{"key":"control","value":"control-value","split":1},{"key":"treatment","value":"treatment-value","split":0}],
				"rollout":[{"rollout_percentage":1}],
				"test":{"users":{"$company:acme|$user:user-123":"treatment"}}}}`
	}

	t.Run("override matches the distinct id the template builds", func(t *testing.T) {
		provider, exposures := newDefinitionsProvider(t, true, definition(templateField(hgTemplate)))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)
		require.Equal(t, "treatment-value", result)
		require.Len(t, *exposures, 1)
		require.Equal(t, true, (*exposures)[0].props["$is_qa_tester"])
	})

	t.Run("override matches distinct id as sent", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true, definition(""))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback",
			FlagContext{"distinct_id": "$company:acme|$user:user-123", "$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)
		require.Equal(t, "treatment-value", result)
	})

	t.Run("template does not build a distinct id for another context property", func(t *testing.T) {
		provider, _ := newDefinitionsProvider(t, true,
			strings.Replace(definition(templateField(hgTemplate)), `"context":"distinct_id"`, `"context":"account"`, 1))

		result, err := provider.GetVariantValue(ctx, "flag", "fallback", FlagContext{"$company_id": "acme", "$user_id": "user-123"})
		require.NoError(t, err)
		require.Equal(t, "control-value", result)
	})
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

func TestLocalFeatureFlagsProvider_WithoutContextTemplateUnchanged(t *testing.T) {
	ctx := context.Background()
	provider, exposures := newDefinitionsProvider(t, false, hgTestDefinition,
		strings.Replace(strings.Replace(hgTestDefinition, `"context_template":"$company:{$company_id}|$user:{$user_id}",`, "", 1),
			`"key":"hg-test"`, `"key":"plain"`, 1))

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
