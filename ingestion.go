package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

// Track calls the Track endpoint
// For server side we recommend Import func
// more info here: https://developer.mixpanel.com/reference/track-event#when-to-use-track-vs-import
func (m *Mixpanel) Track(ctx context.Context, events []*Event) error {
	response, err := m.doRequest(
		ctx,
		events,
		m.baseEndpoint+trackURL,
		false,
		false,
		None,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to call track event: %w", err)
	}
	defer response.Body.Close()

	return returnVerboseError(response)
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

type ImportGenericError struct {
	Code     int         `json:"code"`
	ApiError string      `json:"error"`
	Status   interface{} `json:"status"`
}

func (e ImportGenericError) Error() string {
	return e.ApiError
}

// Import calls the Import api
// https://developer.mixpanel.com/reference/import-events
func (m *Mixpanel) Import(ctx context.Context, events []*Event, options ImportOptions) error {

	var values url.Values
	values.Add("strict", strconv.FormatBool(options.Strict))
	values.Add("project_id", strconv.Itoa(m.projectID))

	httpResponse, err := m.doRequest(
		ctx,
		events,
		importURL,
		true,
		true,
		options.Compression,
		values,
	)
	if err != nil {
		return fmt.Errorf("failed to import:%w", err)
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
func (m *Mixpanel) PeopleSet(ctx context.Context, distinctID string, properties map[string]any, ip *net.IP) error {
	payload := []peopleSetPayload{
		{
			Token:      m.token,
			DistinctID: distinctID,
			Set:        properties,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleSetURL, ip)
}

type peopleSetOncePayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	SetOnce    map[string]any `json:"$set_once"`
}

func (m *Mixpanel) PeopleSetOnce(ctx context.Context, distinctID string, properties map[string]any, ip *net.IP) error {
	payload := []peopleSetOncePayload{
		{
			Token:      m.token,
			DistinctID: distinctID,
			SetOnce:    properties,
		},
	}
	return m.doPeopleRequest(ctx, payload, peopleSetOnceURL, ip)
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
	return m.doPeopleRequest(ctx, payload, peopleIncrementUrl, nil)
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
	return m.doPeopleRequest(ctx, payload, peopleAppendToListUrl, nil)
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
	return m.doPeopleRequest(ctx, payload, peopleRemoveFromListUrl, nil)
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
	return m.doPeopleRequest(ctx, payload, peopleDeletePropertyUrl, nil)
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
	return m.doPeopleRequest(ctx, payload, peopleBatchUpdateUrl, nil)
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
	return m.doPeopleRequest(ctx, payload, peopleDeleteProfileUrl, nil)
}

type groupUpdateProperty struct {
	Token    string         `json:"token"`
	GroupKey string         `json:"$group_key"`
	GroupId  string         `json:"$group_id"`
	Set      map[string]any `json:"$set"`
}

func (m *Mixpanel) GroupSet(ctx context.Context, groupKey, groupID string, set map[string]any) error {
	payload := []groupUpdateProperty{
		{
			Token:    m.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Set:      set,
		},
	}
	return m.doPeopleRequest(ctx, payload, groupSetUrl, nil)
}

type groupSetOnceProperty struct {
	Token    string         `json:"token"`
	GroupKey string         `json:"$group_key"`
	GroupId  string         `json:"$group_id"`
	SetOnce  map[string]any `json:"$set_once"`
}

func (m *Mixpanel) GroupSetOnce(ctx context.Context, groupKey, groupID string, set map[string]any) error {
	payload := []groupSetOnceProperty{
		{
			Token:    m.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			SetOnce:  set,
		},
	}
	return m.doPeopleRequest(ctx, payload, groupsSetOnceUrl, nil)
}

type groupDeleteProperty struct {
	Token    string   `json:"token"`
	GroupKey string   `json:"$group_key"`
	GroupId  string   `json:"$group_id"`
	Unset    []string `json:"$unset"`
}

func (m *Mixpanel) GroupDeleteProperty(ctx context.Context, groupKey, groupID string, unset []string) error {
	payload := []groupDeleteProperty{
		{
			Token:    m.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Unset:    unset,
		},
	}
	return m.doPeopleRequest(ctx, payload, groupsDeletePropertyUrl, nil)
}

type groupRemoveListProperty struct {
}

type groupDelete struct {
	Token    string `json:"token"`
	GroupKey string `json:"$group_key"`
	GroupId  string `json:"$group_id"`
	Delete   string `json:"$delete"`
}

func (m *Mixpanel) GroupDelete(ctx context.Context, groupKey, groupID string) error {
	payload := []groupDelete{
		{
			Token:    m.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Delete:   "null",
		},
	}

	return m.doPeopleRequest(ctx, payload, groupsDeleteGroupUrl, nil)
}

type LookupTable struct {
	Code    int                  `json:"code"`
	Status  string               `json:"status"`
	Results []LookupTableResults `json:"results"`
}

type LookupTableResults struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LookupTableError struct {
	Code     int         `json:"code"`
	ApiError string      `json:"error"`
	Status   interface{} `json:"status"`
}

func (e LookupTableError) Error() string {
	return e.ApiError
}

func (m *Mixpanel) ListLookupTables(ctx context.Context) (*LookupTable, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseEndpoint+lookupUrl+"?project_id="+strconv.Itoa(m.projectID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request:%w", err)
	}

	req.Header.Add(acceptHeader, acceptJsonHeader)

	if m.serviceAccount != nil {
		req.SetBasicAuth(m.serviceAccount.Username, m.serviceAccount.Secret)
	} else {
		return nil, fmt.Errorf("need service account")
	}

	httpResponse, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call lookup table: %w", err)
	}
	defer httpResponse.Body.Close()

	switch httpResponse.StatusCode {
	case http.StatusOK:
		var result LookupTable
		if err := json.NewDecoder(httpResponse.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode results: %w", err)
		}
		return &result, nil
	case http.StatusUnauthorized:
		var e LookupTableError
		if err := json.NewDecoder(httpResponse.Body).Decode(&e); err != nil {
			return nil, fmt.Errorf("failed to decode error response:%w", err)
		}
		return nil, e
	default:
		return nil, ErrStatusCode
	}
}
