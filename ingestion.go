package mixpanel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const (
	trackEventLimit = 50
)

const (
	trackURL = "/track?verbose=1"
)

var (
	ErrTrackToManyEvents = errors.New("track only supports #50 events")
)

// Track calls the /track endpoint
// For server side we recommend /import
// more info here: https://developer.mixpanel.com/reference/track-event#when-to-use-track-vs-import
func (m *Mixpanel) Track(ctx context.Context, events []*Event) error {
	if len(events) > trackEventLimit {
		return ErrTrackToManyEvents
	}

	body, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to create http body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseEndpoint+trackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add(acceptHeader, acceptHeaderValue)
	req.Header.Add(contentType, contentTypeJson)

	httpResponse, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post /track request: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("non 200 status code")
	}

	var r ApiError
	if err := json.NewDecoder(httpResponse.Body).Decode(&r); err != nil {
		return fmt.Errorf("failed to json decode response body: %w", err)
	}

	if r.Status == 0 {
		return r
	}

	return nil
}
