package flags

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	if !ok {
		log.Printf("Failed to track exposure since distinct_id missing or not a string")
		return
	}
	if p.tracker == nil {
		log.Printf("Failed to track exposure since tracker is nil")
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

// callFlagsEndpoint makes an HTTP GET request to a flags API endpoint.
// Returns raw response body for the caller to decode.
func (p *featureFlagsProvider) callFlagsEndpoint(ctx context.Context, path string, additionalParams url.Values) ([]byte, error) {
	u := url.URL{
		Scheme: "https",
		Host:   p.apiHost,
		Path:   path,
	}

	q := u.Query()
	q.Set("token", p.token)
	q.Set("mp_lib", goLib)
	q.Set("$lib_version", p.version)

	for key, values := range additionalParams {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if traceparent, err := generateTraceparent(); err == nil {
		req.Header.Set("traceparent", traceparent)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(p.token + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", cerr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
