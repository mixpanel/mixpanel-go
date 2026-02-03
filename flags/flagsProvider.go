package flags

import (
	"net/http"
	"time"
)

// The featureFlagsProvider contains common fields and methods shared by providers.
// i.e LocalFeatureFlagsProvider and RemoteFeatureFlagsProvider
type featureFlagsProvider struct {
	token          string
	apiHost        string
	version        string
	evaluationMode string
	tracker        Tracker
	client         *http.Client
}

// Manually tracks a feature flag exposure event to Mixpanel.
func (p *featureFlagsProvider) trackExposure(flagKey string, variant SelectedVariant, flagContext FlagContext, latency *time.Duration) {
	distinctID, ok := flagContext["distinct_id"].(string)
	if !ok || p.tracker == nil {
		return
	}

	properties := map[string]any{
		"Experiment name":      flagKey,
		"Variant name":         variant.VariantKey,
		"$experiment_type":     "feature_flag",
		"Flag evaluation mode": p.evaluationMode,
	}

	if variant.ExperimentID != nil {
		properties["$experiment_id"] = *variant.ExperimentID
	}
	if variant.IsExperimentActive != nil {
		properties["$is_experiment_active"] = *variant.IsExperimentActive
	}
	if variant.IsQATester != nil {
		properties["$is_qa_tester"] = *variant.IsQATester
	}
	if latency != nil {
		properties["Variant fetch latency (ms)"] = float64(latency.Milliseconds())
	}

	p.tracker(distinctID, exposureEventName, properties)
}
