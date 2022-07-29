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
	t.Run("tack multiple event", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		mp := NewClient("token")
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

	t.Run("test len", func(t *testing.T) {
		ctx := context.Background()

		mp := NewClient("token")
		var events []*Event

		for i := 0; i < trackEventLimit+1; i++ {
			events = append(events, mp.NewEvent(fmt.Sprintf("event %d", i), EmptyDistinctID, map[string]any{}))
		}
		err := mp.Track(ctx, events)
		require.ErrorIs(t, err, ErrTrackToManyEvents)
	})
}
