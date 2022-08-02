package mixpanel

import (
	"context"
	"errors"
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

var (
	ErrInvalidDistinctID = errors.New("invalid distinct_id")
	ErrReservedProperty  = errors.New("reserved property is set")

	Recommend MpCompression = 0
	None      MpCompression = 0
	Gzip      MpCompression = 1
)

type MpCompression int

// Event is a mixpanel event: https://help.mixpanel.com/hc/en-us/articles/360041995352-Mixpanel-Concepts-Events
type Event struct {
	Name       string         `json:"event"`
	Properties map[string]any `json:"properties"`
}

type Ingestion interface {
	Track(ctx context.Context, events []*Event) error
	Import(ctx context.Context, events []*Event, options ImportOptions) error

	PeopleSet(ctx context.Context, distinctID string, properties map[string]any) error
}

var _ Ingestion = (*Mixpanel)(nil)

type MixpanelServiceAccount struct {
	Username string
	Secret   string
}

type Mixpanel struct {
	client       *http.Client
	baseEndpoint string

	projectID int
	token     string
	apiSecret string

	serviceAccount *MixpanelServiceAccount
}

type Options func(mixpanel *Mixpanel)

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
		mixpanel.serviceAccount = &MixpanelServiceAccount{
			Username: username,
			Secret:   secret,
		}
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
