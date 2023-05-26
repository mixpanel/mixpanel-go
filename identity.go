package mixpanel

import "context"

const (
	identityEndpoint = "/track#create-identity"
	aliasEndpoint    = "/track#identity-create-alias"
	mergeEndpoint    = "/import"
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
func (m *Mixpanel) Alias(ctx context.Context, aliasID, distinctID string) error {
	payload := &aliasPayload{
		Event: "$create_alias",
		Properties: aliasProperties{
			DistinctId: distinctID,
			Alias:      aliasID,
			Token:      m.token,
		},
	}

	return m.doIdentifyRequest(ctx, payload, aliasEndpoint)
}

type mergePayload struct {
	Event      string          `json:"event"`
	Properties mergeProperties `json:"properties"`
}

type mergeProperties struct {
	DistinctId []string `json:"$distinct_ids"`
}

// https://developer.mixpanel.com/reference/identity-merge
// must provide api secret
func (m *Mixpanel) Merge(ctx context.Context, distinctID1, distinctID2 string) error {
	payload := &mergePayload{
		Event: "$merge",
		Properties: mergeProperties{
			DistinctId: []string{distinctID1, distinctID2},
		},
	}

	return m.doIdentifyRequest(ctx, payload, mergeEndpoint, m.useApiSecret())
}
