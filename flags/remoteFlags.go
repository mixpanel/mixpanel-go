package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// RemoteFeatureFlagsProvider evaluates feature flags via server-side API requests
type RemoteFeatureFlagsProvider struct {
	featureFlagsProvider

	config RemoteFlagsConfig
}

// NewRemoteFeatureFlagsProvider creates a new remote feature flags provider
func NewRemoteFeatureFlagsProvider(token string, config RemoteFlagsConfig, tracker Tracker) *RemoteFeatureFlagsProvider {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: config.RequestTimeout,
		}
	}

	if config.APIHost == "" {
		config.APIHost = defaultFlagsAPIHost
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = defaultRequestTimeout
	}

	return &RemoteFeatureFlagsProvider{
		featureFlagsProvider: featureFlagsProvider{
			token:          token,
			apiHost:        config.APIHost,
			version:        version,
			evaluationMode: "remote",
			tracker:        tracker,
			client:         client,
		},
		config: config,
	}
}

// GetVariantValue returns the variant value for a flag from the remote server
func (p *RemoteFeatureFlagsProvider) GetVariantValue(ctx context.Context, flagKey string, fallbackValue any, flagContext FlagContext) (any, error) {
	variant, err := p.GetVariant(ctx, flagKey, SelectedVariant{VariantValue: fallbackValue}, flagContext, true)
	if err != nil {
		return fallbackValue, err
	}
	return variant.VariantValue, nil
}

// IsEnabled returns true if the flag is enabled (variant value is exactly true)
// This should ONLY be used for FeatureGate flags
func (p *RemoteFeatureFlagsProvider) IsEnabled(ctx context.Context, flagKey string, flagContext FlagContext) (bool, error) {
	value, err := p.GetVariantValue(ctx, flagKey, false, flagContext)
	if err != nil {
		return false, err
	}
	return value == true, nil
}

// GetVariant returns the complete variant for a flag from the remote server
func (p *RemoteFeatureFlagsProvider) GetVariant(ctx context.Context, flagKey string, fallbackVariant SelectedVariant, flagContext FlagContext, reportExposure bool) (SelectedVariant, error) {
	startTime := time.Now()

	response, err := p.fetchFlags(ctx, flagContext, &flagKey)
	if err != nil {
		return fallbackVariant, fmt.Errorf("failed to fetch flags: %w", err)
	}

	latency := time.Since(startTime)

	selectedVariant, ok := response.Flags[flagKey]
	if !ok || selectedVariant == nil {
		return fallbackVariant, nil
	}

	if reportExposure {
		p.trackExposure(flagKey, *selectedVariant, flagContext, &latency)
	}

	return *selectedVariant, nil
}

// GetAllVariants returns all flag variants for the context from the remote server
func (p *RemoteFeatureFlagsProvider) GetAllVariants(ctx context.Context, flagContext FlagContext) (map[string]SelectedVariant, error) {
	response, err := p.fetchFlags(ctx, flagContext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch flags: %w", err)
	}

	result := make(map[string]SelectedVariant)
	for key, variant := range response.Flags {
		if variant != nil {
			result[key] = *variant
		}
	}

	return result, nil
}

// TrackExposureEvent manually tracks an exposure event
func (p *RemoteFeatureFlagsProvider) TrackExposureEvent(ctx context.Context, flagKey string, variant SelectedVariant, flagContext FlagContext) {
	p.trackExposure(flagKey, variant, flagContext, nil)
}

func (p *RemoteFeatureFlagsProvider) fetchFlags(ctx context.Context, flagContext FlagContext, flagKey *string) (*remoteFlagsResponse, error) {
	additionalParams := url.Values{}

	contextJSON, err := json.Marshal(flagContext)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal context: %w", err)
	}
	additionalParams.Set("context", url.QueryEscape(string(contextJSON)))

	if flagKey != nil {
		additionalParams.Set("flag_key", *flagKey)
	}

	body, err := p.callFlagsEndpoint(ctx, flagsURLPath, additionalParams)
	if err != nil {
		return nil, err
	}

	var result remoteFlagsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
