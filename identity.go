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

func (m *Mixpanel) Alias(ctx context.Context, aliasID, distinctID string) error {
	payload := &aliasPayload{
		Event: "$create_alias",
		Properties: aliasProperties{
			DistinctId: distinctID,
			Alias:      aliasID,
			Token:      m.token,
		},
	}

	return m.doPeopleRequest(ctx, payload, aliasEndpoint)
}

type mergePayload struct {
	Event      string          `json:"event"`
	Properties mergeProperties `json:"properties"`
}

type mergeProperties struct {
	DistinctId []string `json:"$distinct_ids"`
}

func (m *Mixpanel) Merge(ctx context.Context, distinctID1, distinctID2 string) error {
	payload := &mergePayload{
		Event: "$merge",
		Properties: mergeProperties{
			DistinctId: []string{distinctID1, distinctID2},
		},
	}

	return m.doPeopleRequest(ctx, payload, mergeEndpoint, m.useApiSecret())
}
