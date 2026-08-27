package flags

import (
	"context"
	"encoding/json"
	"net/http"
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
