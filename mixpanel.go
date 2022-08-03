package mixpanel

import (
	"context"
	"net"
	"net/http"
	"time"
)

const (
	version = "v0.0.0"

	usEndpoint = "https://api.mixpanel.com"
	euEndpoint = "https://api-eu.mixpanel.com"

	EmptyDistinctID = ""

	propertyToken      = "token"
	propertyDistinctID = "distinct_id"

	propertyIP         = "ip"
	propertyInsertID   = "$insert_id"
	propertyTime       = "time"
	propertyMpLib      = "mp_lib"
	goLib              = "go"
	propertyLibVersion = "$lib_version"

	acceptHeader          = "Accept"
	acceptPlainTextHeader = "text/plain"
	acceptJsonHeader      = "application/json"
	contentType           = "Content-Type"
	contentTypeJson       = "application/json"
	contentEncodingHeader = "Content-Encoding"
)

type MpCompression int

var (
	None MpCompression = 0
	Gzip MpCompression = 1
)

type Ingestion interface {
	// Events
	Track(ctx context.Context, events []*Event) error
	Import(ctx context.Context, events []*Event, options ImportOptions) (*ImportSuccess, error)

	// People
	PeopleSet(ctx context.Context, distinctID string, properties map[string]any, options ...PeopleOptions) error
	PeopleSetOnce(ctx context.Context, distinctID string, properties map[string]any, options ...PeopleOptions) error
	PeopleIncrement(ctx context.Context, distinctID string, add map[string]int) error
	PeopleUnionProperty(ctx context.Context, distinctID string, union map[string]any) error
	PeopleAppendListProperty(ctx context.Context, distinctID string, append map[string]string) error
	PeopleRemoveListProperty(ctx context.Context, distinctID string, remove map[string]string) error
	PeopleDeleteProperty(ctx context.Context, distinctID string, unset []string) error
	PeopleBatchUpdate(ctx context.Context, updates []PeopleBatchUpdate) error
	PeopleDeleteProfile(ctx context.Context, distinctID string, ignoreAlias bool) error

	// Groups

	// Lookup
}

var _ Ingestion = (*Mixpanel)(nil)

// MpApi is all the API's in the Mixpanel docs
// https://developer.mixpanel.com/reference/overview
type MpApi interface {
	Ingestion
}

type ServiceAccount struct {
	Username string
	Secret   string
}

type Mixpanel struct {
	client       *http.Client
	baseEndpoint string

	projectID int
	token     string
	apiSecret string

	serviceAccount *ServiceAccount

	debugHttp bool
}

type Options func(mixpanel *Mixpanel)

// HttpClient will replace the http.DefaultClient with the provided http.Client
func HttpClient(client *http.Client) Options {
	return func(mixpanel *Mixpanel) {
		mixpanel.client = client
	}
}

// EuResidency sets the mixpanel client to use the eu endpoints
// Use for EU Projects
func EuResidency() Options {
	return func(mixpanel *Mixpanel) {
		mixpanel.baseEndpoint = euEndpoint
	}
}

// ProxyLocation sets the mixpanel client to use the custom base endpoint
// Example: http://locahosthost:8080
func ProxyLocation(proxy string) Options {
	return func(mixpanel *Mixpanel) {
		mixpanel.baseEndpoint = proxy
	}
}

// SetServiceAccount add a service account to the mixpanel client
// https://developer.mixpanel.com/reference/service-accounts-api
func SetServiceAccount(username, secret string) Options {
	return func(mixpanel *Mixpanel) {
		mixpanel.serviceAccount = &ServiceAccount{
			Username: username,
			Secret:   secret,
		}
	}
}

// DebugHttpCalls prints payload information and url information for debugging purposes
func DebugHttpCalls() Options {
	return func(mixpanel *Mixpanel) {
		mixpanel.debugHttp = true
	}
}

// NewClient create a new mixpanel client
func NewClient(projectID int, token, secret string, options ...Options) *Mixpanel {
	mp := &Mixpanel{
		projectID:    projectID,
		client:       http.DefaultClient,
		baseEndpoint: usEndpoint,
		token:        token,
		apiSecret:    secret,
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
func (m *Mixpanel) NewEvent(name string, distinctID string, properties map[string]any) *Event {
	e := &Event{
		Name: name,
	}

	// Todo: deep copy map
	properties[propertyToken] = m.token
	properties[propertyDistinctID] = distinctID
	properties[propertyMpLib] = goLib
	properties[propertyLibVersion] = version
	e.Properties = properties

	return e
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
func (e *Event) AddIP(ip *net.IP) {
	e.Properties[propertyIP] = ip.String()
}
