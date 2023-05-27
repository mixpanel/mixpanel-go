package mixpanel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func setupIdentityEndpoint(t *testing.T, client *ApiClient, endpoint string, testReq func(req *http.Request), testPayload func(body io.Reader), httpResponse *http.Response) {
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)

	httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, endpoint), func(req *http.Request) (*http.Response, error) {
		require.Equal(t, req.Header.Get("content-type"), "application/x-www-form-urlencoded")
		require.Equal(t, req.Header.Get("accept"), "text/plain")

		require.NoError(t, req.ParseForm())
		data := req.Form.Get("data")
		testPayload(strings.NewReader(data))

		return httpResponse, nil
	})
}

func TestAlias(t *testing.T) {
	ctx := context.Background()

	mp := NewClient("token")
	setupIdentityEndpoint(t, mp, aliasEndpoint, func(req *http.Request) {}, func(body io.Reader) {
		payload := &aliasPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&payload))

		require.Equal(t, "$create_alias", payload.Event)
		require.Equal(t, "distinct-id", payload.Properties.DistinctId)
		require.Equal(t, "alias-id", payload.Properties.Alias)
		require.Equal(t, "token", payload.Properties.Token)

	}, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("1")),
	})

	require.NoError(t, mp.Alias(ctx, "alias-id", "distinct-id"))
}

func TestMerge(t *testing.T) {
	ctx := context.Background()

	mp := NewClient("token")
	setupIdentityEndpoint(t, mp, mergeEndpoint, func(req *http.Request) {
		auth := req.Header.Get("authorization")
		require.Equal(t, auth, "Basic "+base64.StdEncoding.EncodeToString([]byte(mp.apiSecret+":")))
	}, func(body io.Reader) {
		payload := &mergePayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&payload))

		require.Equal(t, "$merge", payload.Event)
		require.Equal(t, []string{"distinct-id-1", "distinct-id-2"}, payload.Properties.DistinctId)
	}, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("1")),
	})

	require.NoError(t, mp.Merge(ctx, "distinct-id-1", "distinct-id-2"))
}
