package mixpanel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gohobby/deepcopy"
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

var (
	ErrInvalidDistinctID = errors.New("invalid distinct_id")
	ErrReservedProperty  = errors.New("reserved property is set")

	reservedProperties = map[string]struct{}{
		propertyMpLib:      {},
		propertyLibVersion: {},
	}

	invalidDistinctID = map[string]struct{}{
		"00000000-0000-0000-0000-000000000000": {},
		"anon":                                 {},
		"anonymous":                            {},
		"nil":                                  {},
		"none":                                 {},
		"null":                                 {},
		"n/a":                                  {},
		"na":                                   {},
		"undefined":                            {},
		"unknown":                              {},
		"<nil>":                                {},
		"0":                                    {},
		"-1":                                   {},
		"true":                                 {},
		"false":                                {},
		"[]":                                   {},
		"{}":                                   {},
	}
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

// NewEvent creates a new mixpanel event to track
func (m *Mixpanel) NewEvent(name string, distinctID string, properties map[string]any) *Event {
	e := &Event{
		Name: name,
	}

	copyMap := deepcopy.Map(properties).DeepCopy().(map[string]any)

	copyMap[propertyToken] = m.token
	copyMap[propertyDistinctID] = distinctID
	copyMap[propertyMpLib] = goLib
	copyMap[propertyLibVersion] = version
	e.Properties = copyMap

	return e
}

type EventCheckError struct {
	Err         error
	Description string
}

func (e EventCheckError) Error() string {
	return e.Err.Error()
}

func (m *Mixpanel) NewEventWithChecks(name string, distinctID string, properties map[string]any) (*Event, error) {
	for k := range properties {
		if _, ok := reservedProperties[k]; ok {
			return nil, EventCheckError{
				Err:         ErrReservedProperty,
				Description: fmt.Sprintf("property %s is reserved", k),
			}
		}
	}

	for k := range invalidDistinctID {
		if k == distinctID {
			return nil, EventCheckError{
				Err:         ErrInvalidDistinctID,
				Description: fmt.Sprintf("distinct_id %s is not a valid", k),
			}
		}
	}

	return m.NewEvent(name, distinctID, properties), nil
}

func (e *Event) AddTime(t time.Time) {
	e.Properties[propertyTime] = t.UnixMilli()
}
