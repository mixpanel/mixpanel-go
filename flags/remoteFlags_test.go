package flags

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestRemoteFeatureFlagsProvider_GetVariantValue(t *testing.T) {
	t.Run("returns variant value from server", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		variantKey := "enabled"
		response := remoteFlagsResponse{
			Code: 200,
			Flags: map[string]*SelectedVariant{
				"test-flag": {
					VariantKey:   &variantKey,
					VariantValue: true,
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewJsonResponderOrPanic(200, response))

		ctx := context.Background()
		result, err := provider.GetVariantValue(ctx, "test-flag", false, FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.Equal(t, true, result)
	})

	t.Run("returns fallback when flag not found", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		response := remoteFlagsResponse{
			Code:  200,
			Flags: map[string]*SelectedVariant{},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewJsonResponderOrPanic(200, response))

		ctx := context.Background()
		result, err := provider.GetVariantValue(ctx, "nonexistent", "fallback", FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.Equal(t, "fallback", result)
	})

	t.Run("returns fallback and error on server error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewStringResponder(500, "Internal Server Error"))

		ctx := context.Background()
		result, err := provider.GetVariantValue(ctx, "test-flag", "fallback", FlagContext{"distinct_id": "user1"})
		require.Error(t, err)
		require.Equal(t, "fallback", result)
	})
}

func TestRemoteFeatureFlagsProvider_IsEnabled(t *testing.T) {
	t.Run("returns true when variant value is true", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		variantKey := "enabled"
		response := remoteFlagsResponse{
			Code: 200,
			Flags: map[string]*SelectedVariant{
				"bool-flag": {
					VariantKey:   &variantKey,
					VariantValue: true,
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewJsonResponderOrPanic(200, response))

		ctx := context.Background()
		result, err := provider.IsEnabled(ctx, "bool-flag", FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("returns false when variant value is false", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		variantKey := "disabled"
		response := remoteFlagsResponse{
			Code: 200,
			Flags: map[string]*SelectedVariant{
				"bool-flag": {
					VariantKey:   &variantKey,
					VariantValue: false,
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewJsonResponderOrPanic(200, response))

		ctx := context.Background()
		result, err := provider.IsEnabled(ctx, "bool-flag", FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.False(t, result)
	})
}

func TestRemoteFeatureFlagsProvider_GetAllVariants(t *testing.T) {
	t.Run("returns all variants from server", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		key1 := "v1"
		key2 := "v2"
		response := remoteFlagsResponse{
			Code: 200,
			Flags: map[string]*SelectedVariant{
				"flag-1": {VariantKey: &key1, VariantValue: "value1"},
				"flag-2": {VariantKey: &key2, VariantValue: "value2"},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewJsonResponderOrPanic(200, response))

		ctx := context.Background()
		variants, err := provider.GetAllVariants(ctx, FlagContext{"distinct_id": "user1"})
		require.NoError(t, err)
		require.Len(t, variants, 2)
		require.Contains(t, variants, "flag-1")
		require.Contains(t, variants, "flag-2")
	})

	t.Run("returns error on server failure", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewStringResponder(500, "Internal Server Error"))

		ctx := context.Background()
		variants, err := provider.GetAllVariants(ctx, FlagContext{"distinct_id": "user1"})
		require.Error(t, err)
		require.Nil(t, variants)
	})
}

func TestRemoteFeatureFlagsProvider_ExposureTracking(t *testing.T) {
	t.Run("tracks exposure event with correct properties", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()

		var trackedDistinctID string
		var trackedEventName string
		var trackedProps map[string]any

		tracker := func(distinctID string, eventName string, props map[string]any) {
			trackedDistinctID = distinctID
			trackedEventName = eventName
			trackedProps = props
		}

		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, tracker)

		variantKey := "enabled"
		experimentID := "exp-123"
		isActive := true
		response := remoteFlagsResponse{
			Code: 200,
			Flags: map[string]*SelectedVariant{
				"test-flag": {
					VariantKey:         &variantKey,
					VariantValue:       true,
					ExperimentID:       &experimentID,
					IsExperimentActive: &isActive,
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewJsonResponderOrPanic(200, response))

		ctx := context.Background()
		_, err := provider.GetVariant(ctx, "test-flag", SelectedVariant{}, FlagContext{"distinct_id": "user123"}, true)
		require.NoError(t, err)

		require.Equal(t, "user123", trackedDistinctID)
		require.Equal(t, "$experiment_started", trackedEventName)
		require.Equal(t, "test-flag", trackedProps["Experiment name"])
		require.Equal(t, "feature_flag", trackedProps["$experiment_type"])
		require.Equal(t, "remote", trackedProps["Flag evaluation mode"])
		require.Equal(t, "exp-123", trackedProps["$experiment_id"])
		require.Equal(t, true, trackedProps["$is_experiment_active"])
		require.NotNil(t, trackedProps["Variant fetch latency (ms)"])
	})

	t.Run("does not track exposure when reportExposure is false", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()

		trackCount := 0
		tracker := func(distinctID string, eventName string, props map[string]any) {
			trackCount++
		}

		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, tracker)

		variantKey := "enabled"
		response := remoteFlagsResponse{
			Code: 200,
			Flags: map[string]*SelectedVariant{
				"test-flag": {
					VariantKey:   &variantKey,
					VariantValue: true,
				},
			},
		}

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			httpmock.NewJsonResponderOrPanic(200, response))

		ctx := context.Background()
		_, err := provider.GetVariant(ctx, "test-flag", SelectedVariant{}, FlagContext{"distinct_id": "user123"}, false)
		require.NoError(t, err)
		require.Equal(t, 0, trackCount)
	})
}

func TestRemoteFeatureFlagsProvider_RequestFormat(t *testing.T) {
	t.Run("sends correct query parameters", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		var capturedRequest *http.Request

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			func(req *http.Request) (*http.Response, error) {
				capturedRequest = req
				return httpmock.NewJsonResponse(200, remoteFlagsResponse{
					Code:  200,
					Flags: map[string]*SelectedVariant{},
				})
			})

		ctx := context.Background()
		_, _ = provider.GetVariantValue(ctx, "test-flag", "fallback", FlagContext{"distinct_id": "user1"})

		require.NotNil(t, capturedRequest)

		query := capturedRequest.URL.Query()
		require.Equal(t, "test-token", query.Get("token"))
		require.Equal(t, "go", query.Get("mp_lib"))
		require.NotEmpty(t, query.Get("$lib_version"))
		require.NotEmpty(t, query.Get("context"))
		require.Equal(t, "test-flag", query.Get("flag_key"))

		auth := capturedRequest.Header.Get("Authorization")
		require.Contains(t, auth, "Basic")

		traceparent := capturedRequest.Header.Get("traceparent")
		require.NotEmpty(t, traceparent)
		require.Contains(t, traceparent, "00-")
	})

	t.Run("correctly encodes special characters in context", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		config := DefaultRemoteFlagsConfig()
		provider := NewRemoteFeatureFlagsProvider("test-token", "test", config, nil)

		var capturedRequest *http.Request
		variantKey := "enabled"

		httpmock.RegisterResponder(http.MethodGet, "https://api.mixpanel.com/flags",
			func(req *http.Request) (*http.Response, error) {
				capturedRequest = req
				return httpmock.NewJsonResponse(200, remoteFlagsResponse{
					Code: 200,
					Flags: map[string]*SelectedVariant{
						"test-flag": {
							VariantKey:   &variantKey,
							VariantValue: true,
						},
					},
				})
			})

		specialID := `user&id=1+2 "quoted"`
		flagCtx := FlagContext{"distinct_id": specialID}

		ctx := context.Background()
		result, err := provider.GetVariantValue(ctx, "test-flag", false, flagCtx)
		require.NoError(t, err)
		require.Equal(t, true, result)

		require.NotNil(t, capturedRequest)

		contextParam := capturedRequest.URL.Query().Get("context")
		require.NotEmpty(t, contextParam)

		var decoded FlagContext
		err = json.Unmarshal([]byte(contextParam), &decoded)
		require.NoError(t, err)
		require.Equal(t, specialID, decoded["distinct_id"])
	})
}
