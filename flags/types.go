package flags

import (
	"net/http"
	"time"
)

const (
	goLib = "go"

	exposureEventName       = "$experiment_started"
	flagsDefinitionsURLPath = "/flags/definitions"
	flagsURLPath            = "/flags"
	defaultFlagsAPIHost     = "api.mixpanel.com"
	defaultRequestTimeout   = 10 * time.Second
	defaultPollingInterval  = 60 * time.Second
)

type Tracker func(distinctID string, eventName string, properties map[string]any)

type FlagContext map[string]any

type FlagsConfig struct {
	APIHost        string
	RequestTimeout time.Duration
	HTTPClient     *http.Client
}

type LocalFlagsConfig struct {
	FlagsConfig
	EnablePolling   bool
	PollingInterval time.Duration
}

type RemoteFlagsConfig struct {
	FlagsConfig
}

func DefaultLocalFlagsConfig() LocalFlagsConfig {
	return LocalFlagsConfig{
		FlagsConfig: FlagsConfig{
			APIHost:        defaultFlagsAPIHost,
			RequestTimeout: defaultRequestTimeout,
		},
		EnablePolling:   true,
		PollingInterval: defaultPollingInterval,
	}
}

func DefaultRemoteFlagsConfig() RemoteFlagsConfig {
	return RemoteFlagsConfig{
		FlagsConfig: FlagsConfig{
			APIHost:        defaultFlagsAPIHost,
			RequestTimeout: defaultRequestTimeout,
		},
	}
}

// VariantSource — where a SelectedVariant came from. Set by the providers on
// every returned variant. Coarse-grained (local / remote / fallback); see
// FallbackReason for the specific reason behind a fallback.
const (
	VariantSourceLocal    = "local"
	VariantSourceRemote   = "remote"
	VariantSourceFallback = "fallback"
)

// FallbackReason — why the SDK returned the developer fallback. Only meaningful
// when VariantSource == VariantSourceFallback. Matches the constant set used by
// mixpanel-php so the OpenFeature wrapper can map each reason to the
// spec-correct error code instead of collapsing every fallback to FLAG_NOT_FOUND.
const (
	FallbackReasonFlagNotFound      = "FLAG_NOT_FOUND"
	FallbackReasonMissingContextKey = "MISSING_CONTEXT_KEY"
	FallbackReasonNoRolloutMatch    = "NO_ROLLOUT_MATCH"
	FallbackReasonBackendError      = "BACKEND_ERROR"
)

type SelectedVariant struct {
	VariantKey         *string `json:"variant_key"`
	VariantValue       any     `json:"variant_value"`
	ExperimentID       *string `json:"experiment_id,omitempty"`
	IsExperimentActive *bool   `json:"is_experiment_active,omitempty"`
	IsQATester         *bool   `json:"is_qa_tester,omitempty"`
	VariantSource      string  `json:"variant_source,omitempty"`
	// FallbackReason is empty on success; one of the FallbackReason* constants
	// when VariantSource is VariantSourceFallback.
	FallbackReason string `json:"fallback_reason,omitempty"`
}

// WithSource returns a copy of v tagged with the given source. Clears
// FallbackReason — use AsFallback when returning a fallback.
func (v SelectedVariant) WithSource(source string) SelectedVariant {
	v.VariantSource = source
	v.FallbackReason = ""
	return v
}

// AsFallback returns a copy of v tagged as a fallback with the given reason.
func (v SelectedVariant) AsFallback(reason string) SelectedVariant {
	v.VariantSource = VariantSourceFallback
	v.FallbackReason = reason
	return v
}

type Variant struct {
	Key       string  `json:"key"`
	Value     any     `json:"value"`
	IsControl bool    `json:"is_control"`
	Split     float64 `json:"split"`
}

type VariantOverride struct {
	Key string `json:"key"`
}

type Rollout struct {
	RolloutPercentage     float64            `json:"rollout_percentage"`
	RuntimeEvaluationRule map[string]any     `json:"runtime_evaluation_rule,omitempty"`
	VariantOverride       *VariantOverride   `json:"variant_override,omitempty"`
	VariantSplits         map[string]float64 `json:"variant_splits,omitempty"`
}

type FlagTestUsers struct {
	Users map[string]string `json:"users"`
}

type RuleSet struct {
	Variants []Variant      `json:"variants"`
	Rollout  []Rollout      `json:"rollout"`
	Test     *FlagTestUsers `json:"test,omitempty"`
}

type ExperimentationFlag struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Key                string  `json:"key"`
	Status             string  `json:"status"`
	ProjectID          int     `json:"project_id"`
	Ruleset            RuleSet `json:"ruleset"`
	Context            string  `json:"context"`
	ExperimentID       *string `json:"experiment_id,omitempty"`
	IsExperimentActive *bool   `json:"is_experiment_active,omitempty"`
	HashSalt           *string `json:"hash_salt,omitempty"`
}

type experimentationFlagsResponse struct {
	Flags []ExperimentationFlag `json:"flags"`
}

type remoteFlagsResponse struct {
	Code  int                         `json:"code"`
	Flags map[string]*SelectedVariant `json:"flags"`
}
