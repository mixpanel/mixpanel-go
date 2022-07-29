package mixpanel

import (
	"context"
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
	t.Run("tack 1 event", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		mp := NewClient("token")
		event := mp.NewEvent("sample_event", EmptyDistinctID, map[string]any{})

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, trackURL), func(req *http.Request) (*http.Response, error) {
			var r []eventPost
			require.NoError(t, json.NewDecoder(req.Body).Decode(&r))
			require.Len(t, r, 1)
			require.Equal(t, event.Name, r[0].Name)

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

		require.NoError(t, mp.Track(ctx, []*Event{event}))
	})
}
