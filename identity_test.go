package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestAlias(t *testing.T) {
	t.Run("can call alias", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, aliasEndpoint), func(req *http.Request) (*http.Response, error) {
			require.NoError(t, req.ParseForm())
			data := req.Form.Get("data")
			require.NotEmpty(t, data)

			aliasPost := &aliasPayload{}
			require.NoError(t, json.Unmarshal([]byte(data), aliasPost))
			require.Equal(t, "$create_alias", aliasPost.Event)
			require.Equal(t, "distinct_id", aliasPost.Properties.DistinctId)
			require.Equal(t, "alias_id", aliasPost.Properties.Alias)
			require.Equal(t, "token", aliasPost.Properties.Token)

			body := `
			1
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.Alias(ctx, "alias_id", "distinct_id"))
	})
}

func TestMerge(t *testing.T) {
	t.Run("can call merge", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, mergeEndpoint), func(req *http.Request) (*http.Response, error) {
			require.NoError(t, req.ParseForm())
			data := req.Form.Get("data")
			require.NotEmpty(t, data)

			mergePost := &mergePayload{}
			require.NoError(t, json.Unmarshal([]byte(data), mergePost))
			require.Equal(t, "$merge", mergePost.Event)
			require.Equal(t, []string{"distinct_id1", "distinct_id2"}, mergePost.Properties.DistinctId)

			body := `
			1
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.Merge(ctx, "distinct_id1", "distinct_id2"))
	})
}
