package mixpanel

import (
	"context"
	"errors"
)

const (
	aliasEndpoint = "/track#identity-create-alias"
	mergeEndpoint = "/import"
)

type aliasPayload struct {
	Event      string          `json:"event"`
	Properties aliasProperties `json:"properties"`
}
type aliasProperties struct {
	DistinctId string `json:"distinct_id"`
	Alias      string `json:"alias"`
	Token      string `json:"token"`
}

// https://developer.mixpanel.com/reference/identity-create-alias
func (a *ApiClient) Alias(ctx context.Context, aliasID, distinctID string) error {
	payload := &aliasPayload{
		Event: "$create_alias",
		Properties: aliasProperties{
			DistinctId: distinctID,
			Alias:      aliasID,
			Token:      a.token,
		},
	}

	return a.doIdentifyRequest(ctx, payload, aliasEndpoint)
}

type mergePayload struct {
	Event      string          `json:"event"`
	Properties mergeProperties `json:"properties"`
}

type mergeProperties struct {
	DistinctId []string `json:"$distinct_ids"`
}

// https://developer.mixpanel.com/reference/identity-merge
// must provide a service account
func (a *ApiClient) Merge(ctx context.Context, distinctID1, distinctID2 string) error {
	if a.serviceAccount == nil {
		return errors.New("service account is required for the merge api")
	}

	payload := &mergePayload{
		Event: "$merge",
		Properties: mergeProperties{
			DistinctId: []string{distinctID1, distinctID2},
		},
	}

	return a.doIdentifyRequest(ctx, payload, mergeEndpoint, a.importAuthOptions())
}
