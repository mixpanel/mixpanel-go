package mixpanel

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	version = "v0.1.1"

	usEndpoint     = "https://api.mixpanel.com"
	usDataEndpoint = "https://data.mixpanel.com"

	euEndpoint     = "https://api-eu.mixpanel.com"
	euDataEndpoint = "https://data-eu.mixpanel.com"

	EmptyDistinctID = ""

	propertyToken      = "token"
	propertyDistinctID = "distinct_id"

	propertyIP         = "ip"
	propertyInsertID   = "$insert_id"
	propertyTime       = "time"
	propertyMpLib      = "mp_lib"
	goLib              = "go"
	propertyLibVersion = "$lib_version"

	acceptHeader               = "Accept"
	acceptPlainTextHeader      = "text/plain"
	acceptJsonHeader           = "application/json"
	contentEncodingHeader      = "Content-Encoding"
	contentTypeHeader          = "Content-Type"
	contentTypeApplicationJson = "application/json"
	contentTypeApplicationForm = "application/x-www-form-urlencoded"
)

type Ingestion interface {
	// Events
	Track(ctx context.Context, events []*Event) error
	Import(ctx context.Context, events []*Event, options ImportOptions) (*ImportSuccess, error)

	// People
	PeopleSet(ctx context.Context, people []*PeopleProperties) error
	PeopleSetOnce(ctx context.Context, people []*PeopleProperties) error
	PeopleIncrement(ctx context.Context, distinctID string, add map[string]int) error
	PeopleUnionProperty(ctx context.Context, distinctID string, union map[string]any) error
	PeopleAppendListProperty(ctx context.Context, distinctID string, append map[string]any) error
	PeopleRemoveListProperty(ctx context.Context, distinctID string, remove map[string]any) error
	PeopleDeleteProperty(ctx context.Context, distinctID string, unset []string) error
	PeopleDeleteProfile(ctx context.Context, distinctID string, ignoreAlias bool) error

	// Groups
	GroupSet(ctx context.Context, groupKey, groupID string, set map[string]any) error
	GroupSetOnce(ctx context.Context, groupKey, groupID string, set map[string]any) error
	GroupDeleteProperty(ctx context.Context, groupKey, groupID string, unset []string) error
	GroupRemoveListProperty(ctx context.Context, groupKey, groupID string, remove map[string]any) error
	GroupUnionListProperty(ctx context.Context, groupKey, groupID string, union map[string]any) error
	GroupDelete(ctx context.Context, groupKey, groupID string) error
}

var _ Ingestion = (*ApiClient)(nil)

type Export interface {
	Export(ctx context.Context, fromDate, toDate time.Time, limit int, event, where string) ([]*Event, error)
}

var _ Export = (*ApiClient)(nil)

type Identity interface {
	Alias(ctx context.Context, distinctID, aliasID string) error
	Merge(ctx context.Context, distinctID1, distinctID2 string) error
}

var _ Identity = (*ApiClient)(nil)

// Api is all the API's in the Mixpanel docs
// https://developer.mixpanel.com/reference/overview
type Api interface {
	Ingestion
	Export
	Identity
}

type serviceAccount struct {
	Username string
	Secret   string
}

type ApiClient struct {
	client       *http.Client
	apiEndpoint  string
	dataEndpoint string

	projectID int
	token     string
	apiSecret string

	serviceAccount *serviceAccount
	debugHttpCall  *debugHttpCalls
}

type Options func(mixpanel *ApiClient)

func ApiSecret(apiSecret string) Options {
	return func(mixpanel *ApiClient) {
		mixpanel.apiSecret = apiSecret
	}
}

// HttpClient will replace the http.DefaultClient with the provided http.Client
func HttpClient(client *http.Client) Options {
	return func(mixpanel *ApiClient) {
		mixpanel.client = client
	}
}

// EuResidency sets the mixpanel client to use the eu endpoints
// Use for EU Projects
func EuResidency() Options {
	return func(mixpanel *ApiClient) {
		mixpanel.apiEndpoint = euEndpoint
		mixpanel.dataEndpoint = euDataEndpoint
	}
}

// ProxyApiLocation sets the mixpanel client to use the custom location for all ingestion requests
// Example: http://locahosthost:8080
func ProxyApiLocation(proxy string) Options {
	return func(mixpanel *ApiClient) {
		mixpanel.apiEndpoint = proxy
	}
}

// ProxyDataLocation sets the mixpanel client to use the custom location for all data requests
// Example: http://locahosthost:8080
func ProxyDataLocation(proxy string) Options {
	return func(mixpanel *ApiClient) {
		mixpanel.dataEndpoint = proxy
	}
}

// ServiceAccount add a service account to the mixpanel client
// https://developer.mixpanel.com/reference/service-accounts-api
func ServiceAccount(projectID int, username, secret string) Options {
	return func(mixpanel *ApiClient) {
		mixpanel.projectID = projectID
		mixpanel.serviceAccount = &serviceAccount{
			Username: username,
			Secret:   secret,
		}
	}
}

// DebugHttpCalls streams json payload information and url information for debugging purposes
func DebugHttpCalls(writer io.Writer) Options {
	return func(mixpanel *ApiClient) {
		mixpanel.debugHttpCall = &debugHttpCalls{
			writer: writer,
		}
	}
}

// NewClient create a new mixpanel client
func NewClient(token string, options ...Options) *ApiClient {
	mp := &ApiClient{
		client:        http.DefaultClient,
		apiEndpoint:   usEndpoint,
		dataEndpoint:  usDataEndpoint,
		token:         token,
		debugHttpCall: &debugHttpCalls{},
	}

	for _, o := range options {
		o(mp)
	}

	return mp
}

// Event is a mixpanel event: https://help.mixpanel.com/hc/en-us/articles/360041995352-Mixpanel-Concepts-Events
type Event struct {
	Name       string         `json:"event"`
	Properties map[string]any `json:"properties"`
}

// NewEvent creates a new mixpanel event to track
func (m *ApiClient) NewEvent(name string, distinctID string, properties map[string]any) *Event {
	e := &Event{
		Name: name,
	}
	if properties == nil {
		properties = make(map[string]any)
	}

	properties[propertyToken] = m.token
	properties[propertyDistinctID] = distinctID
	properties[propertyMpLib] = goLib
	properties[propertyLibVersion] = version
	e.Properties = properties

	return e
}

func (m *ApiClient) NewEventFromJson(json map[string]any) (*Event, error) {
	name, ok := json["event"].(string)
	if !ok {
		return nil, errors.New("event name is not a string or is missing")
	}

	properties, ok := json["properties"].(map[string]any)
	if !ok {
		return nil, errors.New("event properties is not a map or is missing")
	}

	return &Event{
		Name:       name,
		Properties: properties,
	}, nil
}

// AddTime insert the time properties into the event
// https://developer.mixpanel.com/reference/import-events#propertiestime
func (e *Event) AddTime(t time.Time) {
	e.Properties[propertyTime] = t.UnixMilli()
}

// AddInsertID inserts the insert_id property into the properties
// https://developer.mixpanel.com/reference/import-events#propertiesinsert_id
func (e *Event) AddInsertID(insertID string) {
	e.Properties[propertyInsertID] = insertID
}

// AddIP if you supply a property ip with an IP address
// Mixpanel will automatically do a GeoIP lookup and replace the ip property with geographic properties (City, Country, Region). These properties can be used in our UI to segment events geographically.
// https://developer.mixpanel.com/reference/import-events#geoip-enrichment
func (e *Event) AddIP(ip net.IP) {
	if ip == nil {
		return
	}
	e.Properties[propertyIP] = ip.String()
}
