package flags

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/diegoholiveira/jsonlogic/v3"
)

// LocalFeatureFlagsProvider evaluates feature flags locally using cached definitions
type LocalFeatureFlagsProvider struct {
	featureFlagsProvider

	config LocalFlagsConfig

	flagDefinitions atomic.Pointer[map[string]*ExperimentationFlag]
	areFlagsReady   atomic.Bool

	stopPolling    chan struct{}
	pollingStopped chan struct{}
	pollingStarted bool
}

// NewLocalFeatureFlagsProvider creates a new local feature flags provider
func NewLocalFeatureFlagsProvider(token string, config LocalFlagsConfig, tracker Tracker) *LocalFeatureFlagsProvider {
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
	if config.PollingInterval == 0 {
		config.PollingInterval = defaultPollingInterval
	}

	provider := &LocalFeatureFlagsProvider{
		featureFlagsProvider: featureFlagsProvider{
			token:          token,
			apiHost:        config.APIHost,
			version:        version,
			evaluationMode: "local",
			tracker:        tracker,
			client:         client,
		},
		config:         config,
		stopPolling:    make(chan struct{}),
		pollingStopped: make(chan struct{}),
	}
	emptyMap := make(map[string]*ExperimentationFlag)
	provider.flagDefinitions.Store(&emptyMap)
	return provider
}

// StartPollingForDefinitions fetches flag definitions immediately and starts background polling if enabled
func (p *LocalFeatureFlagsProvider) StartPollingForDefinitions(ctx context.Context) error {
	if err := p.fetchFlagDefinitions(ctx); err != nil {
		return fmt.Errorf("initial flag definitions fetch failed: %w", err)
	}

	if p.config.EnablePolling && !p.pollingStarted {
		p.pollingStarted = true
		go p.pollForDefinitions(ctx)
	}

	return nil
}

// StopPollingForDefinitions stops the background polling goroutine
func (p *LocalFeatureFlagsProvider) StopPollingForDefinitions() {
	if p.pollingStarted {
		close(p.stopPolling)
		<-p.pollingStopped
		p.pollingStarted = false
	}
}

// AreFlagsReady returns true if flag definitions have been successfully fetched
func (p *LocalFeatureFlagsProvider) AreFlagsReady() bool {
	return p.areFlagsReady.Load()
}

func (p *LocalFeatureFlagsProvider) pollForDefinitions(ctx context.Context) {
	defer close(p.pollingStopped)

	ticker := time.NewTicker(p.config.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopPolling:
			return
		case <-ticker.C:
			if err := p.fetchFlagDefinitions(ctx); err != nil {
				log.Printf("Error polling for flag definitions: %v", err)
			}
		}
	}
}

// GetVariantValue returns the variant value for a flag
func (p *LocalFeatureFlagsProvider) GetVariantValue(ctx context.Context, flagKey string, fallbackValue any, flagContext FlagContext) any {
	variant := p.GetVariant(ctx, flagKey, SelectedVariant{VariantValue: fallbackValue}, flagContext, true)
	return variant.VariantValue
}

// IsEnabled returns true if the flag is enabled (variant value is exactly true)
func (p *LocalFeatureFlagsProvider) IsEnabled(ctx context.Context, flagKey string, flagContext FlagContext) bool {
	value := p.GetVariantValue(ctx, flagKey, false, flagContext)
	return value == true
}

// GetVariant returns the complete variant for a flag
func (p *LocalFeatureFlagsProvider) GetVariant(ctx context.Context, flagKey string, fallbackVariant SelectedVariant, flagContext FlagContext, reportExposure bool) SelectedVariant {
	startTime := time.Now()

	flags := p.flagDefinitions.Load()
	flag, exists := (*flags)[flagKey]

	if !exists {
		return fallbackVariant
	}

	contextValue, ok := flagContext[flag.Context]
	if !ok {
		return fallbackVariant
	}

	var selectedVariant *SelectedVariant

	if testVariant := p.getVariantOverrideForTestUser(flag, flagContext); testVariant != nil {
		selectedVariant = testVariant
	} else if rollout := p.getAssignedRollout(flag, contextValue, flagContext); rollout != nil {
		selectedVariant = p.getAssignedVariant(flag, contextValue, flagKey, rollout)
	}

	if selectedVariant != nil {
		if reportExposure {
			latency := time.Since(startTime)
			p.trackExposure(flagKey, *selectedVariant, flagContext, &latency)
		}
		return *selectedVariant
	}

	return fallbackVariant
}

// GetAllVariants returns all flag variants for the context (no exposure tracking)
func (p *LocalFeatureFlagsProvider) GetAllVariants(ctx context.Context, flagContext FlagContext) map[string]SelectedVariant {
	variants := make(map[string]SelectedVariant)

	flags := p.flagDefinitions.Load()
	flagKeys := make([]string, 0, len(*flags))
	for key := range *flags {
		flagKeys = append(flagKeys, key)
	}

	for _, flagKey := range flagKeys {
		variant := p.GetVariant(ctx, flagKey, SelectedVariant{}, flagContext, false)
		if variant.VariantKey != nil {
			variants[flagKey] = variant
		}
	}

	return variants
}

// TrackExposureEvent manually tracks an exposure event
func (p *LocalFeatureFlagsProvider) TrackExposureEvent(ctx context.Context, flagKey string, variant SelectedVariant, flagContext FlagContext) {
	p.trackExposure(flagKey, variant, flagContext, nil)
}

func (p *LocalFeatureFlagsProvider) getVariantOverrideForTestUser(flag *ExperimentationFlag, flagContext FlagContext) *SelectedVariant {
	if flag.Ruleset.Test == nil || flag.Ruleset.Test.Users == nil {
		return nil
	}

	distinctID, ok := flagContext["distinct_id"].(string)
	if !ok {
		return nil
	}

	variantKey, ok := flag.Ruleset.Test.Users[distinctID]
	if !ok {
		return nil
	}

	return p.getMatchingVariant(variantKey, flag, true)
}

func (p *LocalFeatureFlagsProvider) getAssignedRollout(flag *ExperimentationFlag, contextValue any, flagContext FlagContext) *Rollout {
	for i, rollout := range flag.Ruleset.Rollout {
		var salt string
		if flag.HashSalt != nil {
			salt = flag.Key + *flag.HashSalt + fmt.Sprintf("%d", i)
		} else {
			salt = flag.Key + "rollout"
		}

		rolloutHash := normalizedHash(fmt.Sprintf("%v", contextValue), salt)

		if rolloutHash < rollout.RolloutPercentage && p.isRuntimeEvaluationSatisfied(&rollout, flagContext) {
			return &rollout
		}
	}
	return nil
}

func (p *LocalFeatureFlagsProvider) getAssignedVariant(flag *ExperimentationFlag, contextValue any, flagKey string, rollout *Rollout) *SelectedVariant {
	if rollout.VariantOverride != nil {
		variant := p.getMatchingVariant(rollout.VariantOverride.Key, flag, false)
		if variant != nil {
			return variant
		}
	}

	storedSalt := ""
	if flag.HashSalt != nil {
		storedSalt = *flag.HashSalt
	}
	salt := flagKey + storedSalt + "variant"
	variantHash := normalizedHash(fmt.Sprintf("%v", contextValue), salt)

	variants := make([]Variant, len(flag.Ruleset.Variants))
	copy(variants, flag.Ruleset.Variants)
	sort.Slice(variants, func(i, j int) bool {
		return variants[i].Key < variants[j].Key
	})

	if rollout.VariantSplits != nil {
		for i := range variants {
			if split, ok := rollout.VariantSplits[variants[i].Key]; ok {
				variants[i].Split = split
			}
		}
	}

	var selected *Variant
	cumulative := 0.0
	for i := range variants {
		selected = &variants[i]
		cumulative += variants[i].Split
		if variantHash < cumulative {
			break
		}
	}

	if selected == nil {
		return nil
	}

	return &SelectedVariant{
		VariantKey:         &selected.Key,
		VariantValue:       selected.Value,
		ExperimentID:       flag.ExperimentID,
		IsExperimentActive: flag.IsExperimentActive,
	}
}

func (p *LocalFeatureFlagsProvider) getMatchingVariant(variantKey string, flag *ExperimentationFlag, isQATester bool) *SelectedVariant {
	for _, variant := range flag.Ruleset.Variants {
		if strings.EqualFold(variantKey, variant.Key) {
			sv := &SelectedVariant{
				VariantKey:         &variant.Key,
				VariantValue:       variant.Value,
				ExperimentID:       flag.ExperimentID,
				IsExperimentActive: flag.IsExperimentActive,
			}
			if isQATester {
				isQA := true
				sv.IsQATester = &isQA
			}
			return sv
		}
	}
	return nil
}

func (p *LocalFeatureFlagsProvider) isRuntimeEvaluationSatisfied(rollout *Rollout, flagContext FlagContext) bool {
	if rollout.RuntimeEvaluationRule != nil {
		return p.evaluateJSONLogicRule(rollout.RuntimeEvaluationRule, flagContext)
	}

	return true
}

func (p *LocalFeatureFlagsProvider) evaluateJSONLogicRule(rule map[string]any, flagContext FlagContext) bool {
	customProps, ok := flagContext["custom_properties"].(map[string]any)
	if !ok {
		return false
	}

	normalizedProps := lowercaseKeysAndValues(customProps)
	normalizedRule := lowercaseOnlyLeafNodes(rule)

	result, err := jsonlogic.ApplyInterface(normalizedRule, normalizedProps)
	if err != nil {
		log.Printf("Error evaluating JSON Logic rule: %v", err)
		return false
	}

	switch v := result.(type) {
	case bool:
		return v
	default:
		return false
	}
}

func (p *LocalFeatureFlagsProvider) fetchFlagDefinitions(ctx context.Context) error {
	u := url.URL{
		Scheme: "https",
		Host:   p.apiHost,
		Path:   flagsDefinitionsURLPath,
	}

	q := u.Query()
	q.Set("token", p.token)
	q.Set("mp_lib", goLib)
	q.Set("$lib_version", p.version)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("traceparent", generateTraceparent())

	auth := base64.StdEncoding.EncodeToString([]byte(p.token + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result experimentationFlagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	flags := make(map[string]*ExperimentationFlag)
	for i := range result.Flags {
		flag := &result.Flags[i]
		sort.Slice(flag.Ruleset.Variants, func(a, b int) bool {
			return flag.Ruleset.Variants[a].Key < flag.Ruleset.Variants[b].Key
		})
		flags[flag.Key] = flag
	}

	p.flagDefinitions.Store(&flags)
	p.areFlagsReady.Store(true)

	return nil
}

