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
