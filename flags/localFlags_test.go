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
