package mixpanel

import (
	"context"
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
	propertyTime       = "time"
	propertyMpLib      = "mp_lib"
	goLib              = "go"
	propertyLibVersion = "$lib_version"

	acceptHeader      = "Accept"
	acceptHeaderValue = "text/plain"
	contentType       = "Content-Type"
	contentTypeJson   = "application/json"
)

// Event is a mixpanel event: https://help.mixpanel.com/hc/en-us/articles/360041995352-Mixpanel-Concepts-Events
type Event struct {
	Name       string         `json:"name"`
	Properties map[string]any `json:"properties"`
}

type ApiError struct {
	ApiError string `json:"error"`
	Status   int    `json:"status"`
}

func (a ApiError) Error() string {
	return a.ApiError
}

type IngestionOps interface {
	Apply()
}

type Ingestion interface {
	Track(ctx context.Context, events []*Event) error
}

var _ Ingestion = (*Mixpanel)(nil)

type Mixpanel struct {
	client       *http.Client
	baseEndpoint string

	token string
}

type Options func(mixpanel *Mixpanel)

func HttpClient(client *http.Client) Options {
	return func(mixpanel *Mixpanel) {
		mixpanel.client = client
	}
}

func EuResidency() Options {
	return func(mixpanel *Mixpanel) {
		mixpanel.baseEndpoint = euEndpoint
	}
}

// NewClient create a new mixpanel client
func NewClient(token string, options ...Options) *Mixpanel {
	mp := &Mixpanel{
		client:       http.DefaultClient,
		baseEndpoint: usEndpoint,
		token:        token,
	}

	for _, o := range options {
		o(mp)
	}

	return mp
}

// NewEvent create a new mixpanel event to track
func (m *Mixpanel) NewEvent(name string, distinctID string, properties map[string]any) *Event {
	e := &Event{
		Name:       name,
		Properties: properties,
	}

	e.Properties[propertyToken] = m.token
	e.Properties[propertyDistinctID] = distinctID
	e.Properties[propertyMpLib] = goLib
	e.Properties[propertyLibVersion] = version

	return e
}

func (e *Event) AddTime(t time.Time) {
	e.Properties[propertyTime] = t.UnixMilli()
}
