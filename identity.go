package mixpanel

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const (
	identityEndpoint = "/track#create-identity"
)

type IdentityEvent struct {
	*Event
}

func (m *Mixpanel) NewIdentityEvent(distinctID string, properties map[string]any, identifiedId, anonId string) *IdentityEvent {
	event := m.NewEvent("$identify", distinctID, properties)
	i := &IdentityEvent{
		Event: event,
	}
	i.SetIdentifiedId(identifiedId)
	i.SetAnonId(anonId)

	return i
}

func (i *IdentityEvent) IdentifiedId() any {
	return i.Properties["$identified_id"]
}

// A distinct_id to merge with the $anon_id.
func (i *IdentityEvent) SetIdentifiedId(id string) {
	i.Properties["$identified_id"] = id
}

// A distinct_id to merge with the $identified_id. The $anon_id must be UUID v4 format and not already merged to an $identified_id.
func (i *IdentityEvent) SetAnonId(id string) {
	i.Properties["$anon_id"] = id
}

func (i *IdentityEvent) AnonId(id string) {
	i.Properties["$anon_id"] = id
}

type IdentityOptions struct {
	Strict bool
}

func (m *Mixpanel) CreateIdentity(ctx context.Context, events []*IdentityEvent, identityOptions IdentityOptions) error {
	// todo support import option
	// todo add strict option

	query := url.Values{}
	query.Add("verbose", "1")

	response, err := m.doRequest(
		ctx,
		http.MethodPost,
		m.apiEndpoint+identityEndpoint,
		events,
		None,
		addQueryParams(query), acceptPlainText(),
	)
	if err != nil {
		return fmt.Errorf("failed to create identity:%w", err)
	}
	defer response.Body.Close()

	return returnVerboseError(response)
}
