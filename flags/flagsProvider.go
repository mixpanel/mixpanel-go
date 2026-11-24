package flags

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mixpanel/mixpanel-go/v2/internal/combinedid"
)

// The featureFlagsProvider contains common fields and methods shared by providers.
// i.e LocalFeatureFlagsProvider and RemoteFeatureFlagsProvider
type featureFlagsProvider struct {
	token            string
	apiHost          string
	version          string
	evaluationMode   string
	tracker          Tracker
	client           *http.Client
	exposureExecutor func(send func())
	// hierarchicalGroups changes how exposure events are attributed; it does not change evaluation.
	hierarchicalGroups bool
}

// Manually tracks a feature flag exposure event to Mixpanel.
func (p *featureFlagsProvider) trackExposure(flagKey string, variant SelectedVariant, flagContext FlagContext, latency *time.Duration) {
	var distinctID string
	var hierarchicalProps map[string]any
	if p.hierarchicalGroups {
		var ok bool
		distinctID, hierarchicalProps, ok = hierarchicalExposureIdentity(flagContext)
		if !ok {
			log.Printf("Failed to track exposure since $company_id missing or empty")
			return
		}
		if conflict := combinedDistinctIDConflict(distinctID, hierarchicalProps); conflict != "" {
			log.Printf("Failed to track exposure since %s; exposure dropped", conflict)
			return
		}
	} else {
		var ok bool
		distinctID, ok = flagContext["distinct_id"].(string)
		if !ok {
			log.Printf("Failed to track exposure since distinct_id missing or not a string")
			return
		}
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
	for k, v := range hierarchicalProps {
		properties[k] = v
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

	send := func() { p.tracker(distinctID, exposureEventName, properties) }
	if p.exposureExecutor != nil {
		p.exposureExecutor(send)
		return
	}
	send()
}

// hierarchicalExposureIdentity returns how a hierarchical groups exposure is attributed: the
// context's distinct_id ("" when it has none) and its $user_id and $company_id. A $user_id or
// $company_id that is empty or only whitespace is treated as absent. ok is false when the context
// has neither a $company_id nor a distinct_id, so the event cannot be attributed.
func hierarchicalExposureIdentity(flagContext FlagContext) (distinctID string, props map[string]any, ok bool) {
	props = map[string]any{}
	for _, key := range []string{"$user_id", "$company_id"} {
		if value, present := contextString(flagContext, key); present && strings.TrimSpace(value) != "" {
			props[key] = value
		}
	}
	distinctID, hasDistinctID := contextString(flagContext, "distinct_id")
	_, hasCompanyID := props["$company_id"]
	return distinctID, props, hasCompanyID || hasDistinctID
}

// contextString returns a context value as a string, and false when it is missing, nil or empty.
func contextString(flagContext FlagContext, key string) (string, bool) {
	value, ok := flagContext[key]
	if !ok || value == nil {
		return "", false
	}
	s := fmt.Sprintf("%v", value)
	return s, s != ""
}

// combinedDistinctIDConflict describes how a distinct id of the form $company:{company}|$user:{user}
// names a different company or user than the exposure's $company_id or $user_id, or returns ""
// when they agree, a part is absent, or the distinct id has another form. Ids are compared as
// strings.
func combinedDistinctIDConflict(distinctID string, props map[string]any) string {
	companyID, userID, found := combinedid.Parse(distinctID)
	if !found {
		return ""
	}
	for _, part := range []struct{ name, key, want string }{
		{"company", "$company_id", companyID},
		{"user", "$user_id", userID},
	} {
		got, present := props[part.key]
		if !present {
			continue
		}
		if s := fmt.Sprint(got); s != part.want {
			return fmt.Sprintf("distinct_id %q names %s %q, but %s is %q", distinctID, part.name, part.want, part.key, s)
		}
	}
	return ""
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

	return body, err
}
