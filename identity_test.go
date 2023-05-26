package mixpanel

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlias(t *testing.T) {
	ctx := context.Background()

	mp := NewClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, aliasEndpoint, func(body io.Reader) {
		payload := &aliasPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&payload))

		require.Equal(t, "$create_alias", payload.Event)
		require.Equal(t, "distinct-id", payload.Properties.DistinctId)
		require.Equal(t, "alias-id", payload.Properties.Alias)
		require.Equal(t, "token", payload.Properties.Token)

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.Alias(ctx, "alias-id", "distinct-id"))
}

func TestMerge(t *testing.T) {
	ctx := context.Background()

	mp := NewClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, mergeEndpoint, func(body io.Reader) {
		payload := &mergePayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&payload))

		require.Equal(t, "$merge", payload.Event)
		require.Equal(t, []string{"alias-id", "distinct-id"}, payload.Properties.DistinctId)

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.Merge(ctx, "alias-id", "distinct-id"))
}
