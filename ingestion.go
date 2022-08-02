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
	trackURL  = "/track?verbose=1"
	importURL = "/import"

	// People urls
	peopleSetURL            = "/engage#profile-set"
	peopleSetOnceURL        = "/engage#profile-set-once"
	peopleIncrementUrl      = "/engage#profile-numerical-add"
	peopleUnionToListUrl    = "/engage#profile-union"
	peopleAppendToListUrl   = "/engage#profile-list-append"
	peopleRemoveFromListUrl = "/engage#profile-list-remove"
	peopleBatchUpdateUrl    = "/engage#profile-batch-update"
	peopleDeletePropertyUrl = "/engage#profile-unset"
	peopleDeleteProfileUrl  = "/engage#profile-delete"

	// Group urls
	groupSetUrl                     = "/groups#group-set"
	groupsSetOnceUrl                = "/groups#group-set-once"
	groupsDeletePropertyUrl         = "/groups#group-unset"
	groupsRemoveFromListPropertyUrl = "/groups#group-remove-from-list"
	groupsUnionListPropertyUrl      = "/groups#group-union"
	groupsBatchGroupProfilesUrl     = "/groups#group-batch-update"
	groupsDeleteGroupUrl            = "/groups#group-delete"

	// Lookup tables
	lookupUrl = "/lookup-tables"
)

var (
	ErrTrackToManyEvents = errors.New("track only supports #50 events")
)

// Track calls the Track endpoint
// For server side we recommend Import func
// more info here: https://developer.mixpanel.com/reference/track-event#when-to-use-track-vs-import
func (m *Mixpanel) Track(ctx context.Context, events []*Event) error {
	response, err := m.doBasicRequest(ctx, events, m.baseEndpoint+trackURL, false, false)
	if err != nil {
		return fmt.Errorf("failed to call track event: %w", err)
	}
	defer response.Body.Close()

	return returnVerboseError(response)
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
	payload := []peopleSetPayload{
		{
			Token:      m.token,
			DistinctID: distinctID,
			Set:        properties,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleSetURL)
}

type peopleSetOncePayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	SetOnce    map[string]any `json:"$set_once"`
}

func (m *Mixpanel) PeopleSetOnce(ctx context.Context, distinctID string, properties map[string]any) error {
	payload := []peopleSetOncePayload{
		{
			Token:      m.token,
			DistinctID: distinctID,
			SetOnce:    properties,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleSetOnceURL)
}

type peopleNumericalProperty struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Add        map[string]int `json:"$add"`
}

func (m *Mixpanel) PeopleIncrement(ctx context.Context, distinctID string, add map[string]int) error {
	payload := []peopleNumericalProperty{
		{
			Token:      m.token,
			DistinctID: distinctID,
			Add:        add,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleIncrementUrl)
}

type peopleAppendListProperty struct {
	Token      string            `json:"$token"`
	DistinctID string            `json:"$distinct_id"`
	Append     map[string]string `json:"$append"`
}

func (m *Mixpanel) PeopleAppendListProperty(ctx context.Context, distinctID string, append map[string]string) error {
	payload := []peopleAppendListProperty{
		{
			Token:      m.token,
			DistinctID: distinctID,
			Append:     append,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleAppendToListUrl)
}

type peopleListRemove struct {
	Token      string            `json:"$token"`
	DistinctID string            `json:"$distinct_id"`
	Remove     map[string]string `json:"$remove"`
}

func (m *Mixpanel) PeopleRemoveListProperty(ctx context.Context, distinctID string, remove map[string]string) error {
	payload := []peopleListRemove{
		{
			Token:      m.token,
			DistinctID: distinctID,
			Remove:     remove,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleRemoveFromListUrl)
}

type peopleDeleteProperty struct {
	Token      string   `json:"$token"`
	DistinctID string   `json:"$distinct_id"`
	Unset      []string `json:"$unset"`
}

func (m *Mixpanel) PeopleDeleteProperty(ctx context.Context, distinctID string, unset []string) error {
	payload := []peopleDeleteProperty{
		{
			Token:      m.token,
			DistinctID: distinctID,
			Unset:      unset,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleDeletePropertyUrl)
}

type PeopleBatchUpdate struct {
	DistinctID string         `json:"distinct_id"`
	Add        map[string]any `json:"$add"`
}

type peopleBatchPayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Add        map[string]any `json:"$add"`
}

func (m *Mixpanel) PeopleBatchUpdate(ctx context.Context, updates []PeopleBatchUpdate) error {
	var payload = make([]peopleBatchPayload, 0, len(updates))
	for _, update := range updates {
		payload = append(payload, peopleBatchPayload{
			Token:      m.token,
			DistinctID: update.DistinctID,
			Add:        update.Add,
		})
	}
	return m.doPeopleRequest(ctx, payload, peopleBatchUpdateUrl)
}

type peopleDeleteProfile struct {
	Token       string `json:"$token"`
	DistinctID  string `json:"$distinct_id"`
	Delete      string `json:"$delete"`
	IgnoreAlias string `json:"$ignore_alias"`
}

func (m *Mixpanel) PeopleDeleteProfile(ctx context.Context, distinctID string, ignoreAlias bool) error {
	payload := []peopleDeleteProfile{
		{
			Token:       m.token,
			DistinctID:  distinctID,
			Delete:      "null", // The $delete object value is ignored - the profile is determined by the $distinct_id from the request itself.
			IgnoreAlias: strconv.FormatBool(ignoreAlias),
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleDeleteProfileUrl)
}
