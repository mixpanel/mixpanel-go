package mixpanel

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestTrack(t *testing.T) {
	t.Run("track 1 event", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		mp := NewClient(0, "token", "api-secret")
		events := []*Event{
			mp.NewEvent("sample_event", EmptyDistinctID, map[string]any{}),
		}

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, trackURL), func(req *http.Request) (*http.Response, error) {
			var r []*Event
			require.NoError(t, json.NewDecoder(req.Body).Decode(&r))
			require.Len(t, r, 1)
			require.ElementsMatch(t, events, r)

			body := `
			{
			  "error": "",
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader(body)),
			}, nil
		})

		require.NoError(t, mp.Track(ctx, events))
	})
	t.Run("track multiple event", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		mp := NewClient(0, "token", "api-secret")
		events := []*Event{
			mp.NewEvent("sample_event_1", EmptyDistinctID, map[string]any{}),
			mp.NewEvent("sample_event_2", EmptyDistinctID, map[string]any{}),
			mp.NewEvent("sample_event_3", EmptyDistinctID, map[string]any{}),
		}

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, trackURL), func(req *http.Request) (*http.Response, error) {
			var r []*Event
			require.NoError(t, json.NewDecoder(req.Body).Decode(&r))
			require.Len(t, r, 3)

			require.ElementsMatch(t, events, r)

			body := `
			{
			  "error": "",
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader(body)),
			}, nil
		})

		require.NoError(t, mp.Track(ctx, events))
	})

	t.Run("Error Occurred", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		mp := NewClient(0, "token", "api-secret")

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, trackURL), func(req *http.Request) (*http.Response, error) {
			body := `
			{
			  "error": "",
			  "status": 0
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader(body)),
			}, nil
		})

		err := mp.Track(ctx, []*Event{mp.NewEvent("test-event", EmptyDistinctID, map[string]any{})})
		var g VerboseError
		require.ErrorAs(t, err, &g)
	})
}

func TestImport(t *testing.T) {
	ctx := context.Background()

	t.Run("api-secret-auth", func(t *testing.T) {
		apiSecret := "api-secret-auth"

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get("Authorization")

			require.Equal(t, fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(apiSecret+":"))), authHeader)
			return &http.Response{
				StatusCode: 200,
			}, nil
		})

		mp := NewClient(117, "token", apiSecret)
		require.NoError(t, mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{}))
	})

	t.Run("api-service-account-aut", func(t *testing.T) {
		userName := "username"
		secret := "secret"

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get("Authorization")

			require.Equal(t, fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(userName+":"+secret))), authHeader)
			return &http.Response{
				StatusCode: 200,
			}, nil
		})

		mp := NewClient(117, "token", "api-secret", SetServiceAccount(userName, secret))
		require.NoError(t, mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{}))
	})

	t.Run("api-compression-none", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get(contentEncodingHeader)
			require.Equal(t, "", authHeader)

			data, err := ioutil.ReadAll(req.Body)
			require.NoError(t, err)

			var e []*Event
			require.NoError(t, json.Unmarshal(data, &e))
			require.Len(t, e, 1)

			return &http.Response{
				StatusCode: 200,
			}, nil
		})

		mp := NewClient(117, "token", "auth-secret")
		require.NoError(t, mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Compression: None,
		}))
	})

	t.Run("api-compression-gzip", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get(contentEncodingHeader)
			require.Equal(t, "gzip", authHeader)

			reader, err := gzip.NewReader(req.Body)
			require.NoError(t, err)

			data, err := ioutil.ReadAll(reader)
			require.NoError(t, err)

			var e []*Event
			require.NoError(t, json.Unmarshal(data, &e))
			require.Len(t, e, 1)

			return &http.Response{
				StatusCode: 200,
			}, nil
		})

		mp := NewClient(117, "token", "auth-secret")
		require.NoError(t, mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Compression: Gzip,
		}))
	})

	t.Run("api-strict-enable", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()

			require.Equal(t, "1", query.Get("strict"))

			return &http.Response{
				StatusCode: 200,
			}, nil
		})

		mp := NewClient(117, "token", "auth-secret")
		require.NoError(t, mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Strict: true,
		}))
	})

	t.Run("api-strict-disable", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()

			require.Equal(t, "0", query.Get("strict"))

			return &http.Response{
				StatusCode: 200,
			}, nil
		})

		mp := NewClient(117, "token", "auth-secret")
		require.NoError(t, mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Strict: false,
		}))
	})

	t.Run("api-project-set-correctly", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()

			require.Equal(t, "117", query.Get("project_id"))

			return &http.Response{
				StatusCode: 200,
			}, nil
		})

		mp := NewClient(117, "token", "auth-secret")
		require.NoError(t, mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Strict: false,
		}))
	})

}
