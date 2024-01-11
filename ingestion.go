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
	// https://developer.mixpanel.com/reference/track-event#when-to-use-track-vs-import

	MaxTrackEvents  = 2_000
	MaxImportEvents = 2_000

	// https://developer.mixpanel.com/reference/user-profile-limits

	MaxPeopleEvents = 2_000
)

const (
	trackURL  = "/track"
	importURL = "/import"

	// People urls
	peopleSetURL            = "/engage#profile-set"
	peopleSetOnceURL        = "/engage#profile-set-once"
	peopleIncrementUrl      = "/engage#profile-numerical-add"
	peopleUnionToListUrl    = "/engage#profile-union"
	peopleAppendToListUrl   = "/engage#profile-list-append"
	peopleRemoveFromListUrl = "/engage#profile-list-remove"
	peopleDeletePropertyUrl = "/engage#profile-unset"
	peopleDeleteProfileUrl  = "/engage#profile-delete"

	// Group urls
	groupSetUrl                     = "/groups#group-set"
	groupsSetOnceUrl                = "/groups#group-set-once"
	groupsDeletePropertyUrl         = "/groups#group-unset"
	groupsRemoveFromListPropertyUrl = "/groups#group-remove-from-list"
	groupsUnionListPropertyUrl      = "/groups#group-union"
	groupsDeleteGroupUrl            = "/groups#group-delete"
)

// Track calls the Track endpoint
// For server side we recommend Import func
// more info here: https://developer.mixpanel.com/reference/track-event#when-to-use-track-vs-import
func (m *ApiClient) Track(ctx context.Context, events []*Event) error {
	if len(events) > MaxTrackEvents {
		return fmt.Errorf("max track events is %d", MaxTrackEvents)
	}

	query := url.Values{}
	query.Add("verbose", "1")

	requestBody, err := makeRequestBody(events, jsonPayload, None)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	response, err := m.doRequestBody(
		ctx,
		http.MethodPost,
		m.apiEndpoint+trackURL,
		requestBody,
		addQueryParams(query), acceptPlainText(), applicationJsonHeader(),
	)
	if err != nil {
		return fmt.Errorf("failed to track event: %w", err)
	}
	defer response.Body.Close()

	return parseVerboseApiError(response.Body)
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
	InsertID string `json:"insert_id"`
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

type ImportSuccess struct {
	Code               int         `json:"code"`
	NumRecordsImported int         `json:"num_records_imported"`
	Status             interface{} `json:"status"`
}

type ImportRateLimitError struct {
	ImportGenericError
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
// Need to provide project id a service account, project token or api secret to the client
func (a *ApiClient) Import(ctx context.Context, events []*Event, options ImportOptions) (*ImportSuccess, error) {
	if len(events) > MaxImportEvents {
		return nil, fmt.Errorf("max import events is %d", MaxImportEvents)
	}

	values := url.Values{}
	if options.Strict {
		values.Add("strict", "1")
	} else {
		values.Add("strict", "0")
	}

	if a.serviceAccount != nil {
		values.Add("project_id", strconv.Itoa(a.projectID))
	}
	values.Add("verbose", "1")

	body, err := makeRequestBody(events, jsonPayload, options.Compression)
	if err != nil {
		return nil, fmt.Errorf("failed to create request body: %w", err)
	}

	httpOptions := []httpOptions{applicationJsonHeader(), addQueryParams(values), acceptJson(), a.importAuthOptions()}
	if options.Compression == Gzip {
		httpOptions = append(httpOptions, gzipHeader())
	}

	httpResponse, err := a.doRequestBody(
		ctx,
		http.MethodPost,
		a.apiEndpoint+importURL,
		body,
		httpOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to import:%w", err)
	}
	defer httpResponse.Body.Close()

	switch httpResponse.StatusCode {
	case http.StatusOK:
		var s ImportSuccess
		if err := json.NewDecoder(httpResponse.Body).Decode(&s); err != nil {
			return nil, fmt.Errorf("failed to parse response body:%w", err)
		}
		return &s, nil
	case http.StatusBadRequest:
		var g ImportFailedValidationError
		if err := json.NewDecoder(httpResponse.Body).Decode(&g); err != nil {
			return nil, fmt.Errorf("failed to json decode response body: %w", err)
		}
		return nil, g
	case http.StatusUnauthorized, http.StatusRequestEntityTooLarge:
		var g ImportGenericError
		if err := json.NewDecoder(httpResponse.Body).Decode(&g); err != nil {
			return nil, fmt.Errorf("failed to json decode response body: %w", err)
		}
		return nil, g
	case http.StatusTooManyRequests:
		var g ImportRateLimitError
		if err := json.NewDecoder(httpResponse.Body).Decode(&g); err != nil {
			return nil, fmt.Errorf("failed to json decode response body: %w", err)
		}
		return nil, g
	default:
		return nil, fmt.Errorf("unexpected status code: %d", httpResponse.StatusCode)
	}
}

type PeopleReveredProperties string

const (
	//https://docs.mixpanel.com/docs/tracking/how-tos/user-profiles#reserved-properties

	PeopleEmailProperty           PeopleReveredProperties = "$email"
	PeoplePhoneProperty           PeopleReveredProperties = "$phone"
	PeopleFirstNameProperty       PeopleReveredProperties = "$first_name"
	PeopleLastNameProperty        PeopleReveredProperties = "$last_name"
	PeopleNameProperty            PeopleReveredProperties = "$name"
	PeopleAvatarProperty          PeopleReveredProperties = "$avatar"
	PeopleCreatedProperty         PeopleReveredProperties = "$created"
	PeopleCityProperty            PeopleReveredProperties = "$city"
	PeopleRegionProperty          PeopleReveredProperties = "$region"
	PeopleCountryCodeProperty     PeopleReveredProperties = "$country_code"
	PeopleTimezoneProperty        PeopleReveredProperties = "$timezone"
	PeopleBucketProperty          PeopleReveredProperties = "$bucket"
	PeopleGeolocationByIpProperty PeopleReveredProperties = "$ip"
)

type PeopleProperties struct {
	DistinctID   string
	Properties   map[string]any
	UseRequestIp bool
}

func NewPeopleProperties(distinctID string, properties map[string]any) *PeopleProperties {
	var prop = properties
	if prop == nil {
		prop = make(map[string]any)
	}

	return &PeopleProperties{
		DistinctID: distinctID,
		Properties: prop,
	}
}

func (p *PeopleProperties) SetReservedProperty(property PeopleReveredProperties, value any) {
	p.Properties[string(property)] = value
}

type PeopleIpOptions = func(peopleProperties *PeopleProperties)

func UseRequestIp() PeopleIpOptions {
	return func(peopleProperties *PeopleProperties) {
		peopleProperties.UseRequestIp = true
	}
}

// SetIp will cause mixpanel to geo lookup the ip for the user
// if no ip is provided, we will not lookup
// if ip is provided, we will lookup the ip
// if you want to use the request ip to lookup the geo location then use UseRequestIp option
func (p *PeopleProperties) SetIp(ip net.IP, options ...PeopleIpOptions) {
	if ip != nil {
		p.Properties[string(PeopleGeolocationByIpProperty)] = ip.String()
	}

	for _, option := range options {
		option(p)
	}
}

// Note: if no ip is provided, we will not track by default
func (p *PeopleProperties) shouldGeoLookupIp() string {
	if p.UseRequestIp {
		return ""
	}

	v, ok := p.Properties[string(PeopleGeolocationByIpProperty)]
	if !ok {
		return "0"
	}
	if s, ok := v.(string); ok {
		// if ip is provided, passing it to mixpanel will cause it to be geo lookup
		return s
	}
	return "0"
}

type peopleSetPayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Set        map[string]any `json:"$set"`
	IP         string         `json:"$ip,omitempty"`
}

// PeopleSet calls the User Set Property API
// https://developer.mixpanel.com/reference/profile-set
func (a *ApiClient) PeopleSet(ctx context.Context, people []*PeopleProperties) error {
	if len(people) > MaxPeopleEvents {
		return fmt.Errorf("max people events is %d", MaxPeopleEvents)
	}

	payloads := make([]peopleSetPayload, len(people))
	for i, p := range people {
		payloads[i] = peopleSetPayload{
			Token:      a.token,
			DistinctID: p.DistinctID,
			Set:        p.Properties,
			IP:         p.shouldGeoLookupIp(),
		}
	}

	return a.doPeopleRequest(ctx, payloads, peopleSetURL)
}

type peopleSetOncePayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	SetOnce    map[string]any `json:"$set_once"`
	IP         string         `json:"$ip,omitempty"`
}

// PeopleSetOnce calls the User Set Property Once API
// https://developer.mixpanel.com/reference/profile-set-property-once
func (a *ApiClient) PeopleSetOnce(ctx context.Context, people []*PeopleProperties) error {
	if len(people) > MaxPeopleEvents {
		return fmt.Errorf("max people events is %d", MaxPeopleEvents)
	}

	payloads := make([]peopleSetOncePayload, len(people))
	for i, p := range people {
		payloads[i] = peopleSetOncePayload{
			Token:      a.token,
			DistinctID: p.DistinctID,
			SetOnce:    p.Properties,
			IP:         p.shouldGeoLookupIp(),
		}
	}
	return a.doPeopleRequest(ctx, payloads, peopleSetOnceURL)
}

type peopleNumericalAddPayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Add        map[string]int `json:"$add"`
}

// PeopleIncrement calls the User Increment Numerical Property API
// https://developer.mixpanel.com/reference/profile-numerical-add
func (a *ApiClient) PeopleIncrement(ctx context.Context, distinctID string, add map[string]int) error {
	payload := []peopleNumericalAddPayload{
		{
			Token:      a.token,
			DistinctID: distinctID,
			Add:        add,
		},
	}
	return a.doPeopleRequest(ctx, payload, peopleIncrementUrl)
}

type peopleUnionPayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Union      map[string]any `json:"$union"`
}

// PeopleUnionProperty calls User Union To List Property API
// https://developer.mixpanel.com/reference/user-profile-union
func (a *ApiClient) PeopleUnionProperty(ctx context.Context, distinctID string, union map[string]any) error {
	payload := []peopleUnionPayload{
		{
			Token:      a.token,
			DistinctID: distinctID,
			Union:      union,
		},
	}
	return a.doPeopleRequest(ctx, payload, peopleUnionToListUrl)
}

type peopleAppendListPayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Append     map[string]any `json:"$append"`
}

// PeopleAppend calls the Increment Numerical Property
// https://developer.mixpanel.com/reference/profile-numerical-add
func (a *ApiClient) PeopleAppendListProperty(ctx context.Context, distinctID string, append map[string]any) error {
	payload := []peopleAppendListPayload{
		{
			Token:      a.token,
			DistinctID: distinctID,
			Append:     append,
		},
	}
	return a.doPeopleRequest(ctx, payload, peopleAppendToListUrl)
}

type peopleListRemovePayload struct {
	Token      string         `json:"$token"`
	DistinctID string         `json:"$distinct_id"`
	Remove     map[string]any `json:"$remove"`
}

// PeopleRemoveListProperty calls the User Remove from List Property API
// https://developer.mixpanel.com/reference/profile-remove-from-list-property
func (a *ApiClient) PeopleRemoveListProperty(ctx context.Context, distinctID string, remove map[string]any) error {
	payload := []peopleListRemovePayload{
		{
			Token:      a.token,
			DistinctID: distinctID,
			Remove:     remove,
		},
	}
	return a.doPeopleRequest(ctx, payload, peopleRemoveFromListUrl)
}

type peopleDeletePropertyPayload struct {
	Token      string   `json:"$token"`
	DistinctID string   `json:"$distinct_id"`
	Unset      []string `json:"$unset"`
}

// PeopleDeleteProperty calls the User Delete Property API
// https://developer.mixpanel.com/reference/profile-delete-property
func (a *ApiClient) PeopleDeleteProperty(ctx context.Context, distinctID string, unset []string) error {
	payload := []peopleDeletePropertyPayload{
		{
			Token:      a.token,
			DistinctID: distinctID,
			Unset:      unset,
		},
	}
	return a.doPeopleRequest(ctx, payload, peopleDeletePropertyUrl)
}

type peopleDeleteProfilePayload struct {
	Token       string `json:"$token"`
	DistinctID  string `json:"$distinct_id"`
	Delete      string `json:"$delete"`
	IgnoreAlias string `json:"$ignore_alias"`
}

// PeopleDeleteProfile calls the User Delete Profile API
// https://developer.mixpanel.com/reference/delete-profile
func (a *ApiClient) PeopleDeleteProfile(ctx context.Context, distinctID string, ignoreAlias bool) error {
	payload := []peopleDeleteProfilePayload{
		{
			Token:       a.token,
			DistinctID:  distinctID,
			Delete:      "null", // The $delete object value is ignored - the profile is determined by the $distinct_id from the request itself.
			IgnoreAlias: strconv.FormatBool(ignoreAlias),
		},
	}
	return a.doPeopleRequest(ctx, payload, peopleDeleteProfileUrl)
}

type groupSetPropertyPayload struct {
	Token    string         `json:"$token"`
	GroupKey string         `json:"$group_key"`
	GroupId  string         `json:"$group_id"`
	Set      map[string]any `json:"$set"`
}

// GroupUpdateProperty calls the Group Update Property API
// https://developer.mixpanel.com/reference/group-set-property
func (a *ApiClient) GroupSet(ctx context.Context, groupKey, groupID string, set map[string]any) error {
	payload := []groupSetPropertyPayload{
		{
			Token:    a.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Set:      set,
		},
	}
	return a.doPeopleRequest(ctx, payload, groupSetUrl)
}

type groupSetOncePropertyPayload struct {
	Token    string         `json:"$token"`
	GroupKey string         `json:"$group_key"`
	GroupId  string         `json:"$group_id"`
	SetOnce  map[string]any `json:"$set_once"`
}

// GroupSetOnce calls the Group Set Property Once API
// https://developer.mixpanel.com/reference/group-set-property-once
func (a *ApiClient) GroupSetOnce(ctx context.Context, groupKey, groupID string, set map[string]any) error {
	payload := []groupSetOncePropertyPayload{
		{
			Token:    a.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			SetOnce:  set,
		},
	}
	return a.doPeopleRequest(ctx, payload, groupsSetOnceUrl)
}

type groupDeletePropertyPayload struct {
	Token    string   `json:"$token"`
	GroupKey string   `json:"$group_key"`
	GroupId  string   `json:"$group_id"`
	Unset    []string `json:"$unset"`
}

// GroupDeleteProperty calls the group delete property API
// https://developer.mixpanel.com/reference/group-delete-property
func (a *ApiClient) GroupDeleteProperty(ctx context.Context, groupKey, groupID string, unset []string) error {
	payload := []groupDeletePropertyPayload{
		{
			Token:    a.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Unset:    unset,
		},
	}
	return a.doPeopleRequest(ctx, payload, groupsDeletePropertyUrl)
}

type groupRemoveListPropertyPayload struct {
	Token    string         `json:"$token"`
	GroupKey string         `json:"$group_key"`
	GroupId  string         `json:"$group_id"`
	Remove   map[string]any `json:"$remove"`
}

// GroupRemoveListProperty calls the Groups Remove from List Property API
// https://developer.mixpanel.com/reference/group-remove-from-list-property
func (a *ApiClient) GroupRemoveListProperty(ctx context.Context, groupKey, groupID string, remove map[string]any) error {
	payload := []groupRemoveListPropertyPayload{
		{
			Token:    a.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Remove:   remove,
		},
	}
	return a.doPeopleRequest(ctx, payload, groupsRemoveFromListPropertyUrl)
}

type groupUnionListPropertyPayload struct {
	Token    string         `json:"$token"`
	GroupKey string         `json:"$group_key"`
	GroupId  string         `json:"$group_id"`
	Union    map[string]any `json:"$union"`
}

// GroupUnionListProperty calls the Groups Remove from Union Property API
// https://developer.mixpanel.com/reference/group-union
func (a *ApiClient) GroupUnionListProperty(ctx context.Context, groupKey, groupID string, union map[string]any) error {
	payload := []groupUnionListPropertyPayload{
		{
			Token:    a.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Union:    union,
		},
	}
	return a.doPeopleRequest(ctx, payload, groupsUnionListPropertyUrl)
}

type groupDeletePayload struct {
	Token    string `json:"$token"`
	GroupKey string `json:"$group_key"`
	GroupId  string `json:"$group_id"`
	Delete   string `json:"$delete"`
}

// GroupDelete calls the Groups Delete API
// https://developer.mixpanel.com/reference/delete-group
func (a *ApiClient) GroupDelete(ctx context.Context, groupKey, groupID string) error {
	payload := []groupDeletePayload{
		{
			Token:    a.token,
			GroupKey: groupKey,
			GroupId:  groupID,
			Delete:   "null",
		},
	}

	return a.doPeopleRequest(ctx, payload, groupsDeleteGroupUrl)
}
