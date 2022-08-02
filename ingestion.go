package mixpanel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

const (
	trackURL       = "/track?verbose=1"
	peopleSetURL   = "/engage#profile-set?verbose=1"
	importURL      = "/import"
	apiErrorStatus = 0
)

var (
	ErrTrackToManyEvents = errors.New("track only supports #50 events")
)

type GenericError struct {
	ApiError string `json:"error"`
	Status   int    `json:"status"`
}

func (a GenericError) Error() string {
	return a.ApiError
}

// Track calls the Track endpoint
// For server side we recommend Import func
// more info here: https://developer.mixpanel.com/reference/track-event#when-to-use-track-vs-import
func (m *Mixpanel) Track(ctx context.Context, events []*Event) error {
	body, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to create http body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseEndpoint+trackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add(acceptHeader, acceptPlainTextHeader)
	req.Header.Add(contentType, contentTypeJson)

	return m.executeBasicRequest(req, false)
}

type ImportGenericError struct {
	Code     int         `json:"code"`
	ApiError string      `json:"error"`
	Status   interface{} `json:"status"`
}

func (e ImportGenericError) Error() string {
	return e.ApiError
}

type ImportFailedValidationError struct {
	Code                int                   `json:"code"`
	ApiError            string                `json:"error"`
	Status              interface{}           `json:"status"`
	NumRecordsImported  int                   `json:"num_records_imported"`
	FailedImportRecords []ImportFailedRecords `json:"failed_records"`
}

type ImportFailedRecords struct {
	Index    int    `json:"index"`
	InsertID int    `json:"insert_id"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

func (e ImportFailedValidationError) Error() string {
	return e.ApiError
}

type ImportOptions struct {
	Strict      bool
	Compression MpCompression
}

var ImportOptionsRecommend = ImportOptions{
	Strict:      true,
	Compression: Gzip,
}

// Import calls the Import api
// https://developer.mixpanel.com/reference/import-events
func (m *Mixpanel) Import(ctx context.Context, events []*Event, options ImportOptions) error {
	body, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to create http body: %w", err)
	}

	var contentHeader string
	switch options.Compression {
	case Gzip:
		body, err = gzipBody(body)
		if err != nil {
			return fmt.Errorf("failed to compress: %w", err)
		}
		contentHeader = "gzip"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseEndpoint+importURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add(acceptHeader, acceptJsonHeader)
	req.Header.Add(contentType, contentTypeJson)

	if contentHeader != "" {
		req.Header.Add(contentEncodingHeader, contentHeader)
	}

	if m.serviceAccount != nil {
		req.SetBasicAuth(m.serviceAccount.Username, m.serviceAccount.Secret)
	} else {
		req.SetBasicAuth(m.apiSecret, "")
	}

	query := req.URL.Query()
	query.Set("project_id", strconv.Itoa(m.projectID))
	if options.Strict {
		query.Add("strict", "1")
	} else {
		query.Add("strict", "0")
	}

	req.URL.RawQuery = query.Encode()

	httpResponse, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post /import request: %w", err)
	}
	defer httpResponse.Body.Close()

	switch httpResponse.StatusCode {
	case 200:
		return nil
	case 400:
		var g ImportFailedValidationError
		if err := json.NewDecoder(httpResponse.Body).Decode(&g); err != nil {
			return fmt.Errorf("failed to json decode response body: %w", err)
		}
		return g
	case 401, 413, 429:
		var g ImportGenericError
		if err := json.NewDecoder(httpResponse.Body).Decode(&g); err != nil {
			return fmt.Errorf("failed to json decode response body: %w", err)
		}
		return g
	default:
		return fmt.Errorf("unexpected status code: %d", httpResponse.StatusCode)
	}
}

type peopleSetPayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Set        map[string]any `json:"$set"`
}

// PeopleSet calls the User Set Property API
// https://developer.mixpanel.com/reference/profile-set
func (m *Mixpanel) PeopleSet(ctx context.Context, distinctID string, properties map[string]any) error {
	payload := peopleSetPayload{
		Token:      m.token,
		DistinctID: distinctID,
		Set:        properties,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to create http body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseEndpoint+peopleSetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add(acceptHeader, acceptPlainTextHeader)
	req.Header.Add(contentType, contentTypeJson)

	return m.executeBasicRequest(req, false)
}
