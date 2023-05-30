package mixpanel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func parseDate(t *testing.T, s string) time.Time {
	date, err := time.Parse("2006-01-02", s)
	require.NoError(t, err)
	return date
}

func TestExport(t *testing.T) {
	ctx := context.Background()

	t.Run("can export 2 events", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		queryParams := url.Values{}
		queryParams.Add("from_date", "2023-01-01")
		queryParams.Add("to_date", "2023-01-02")
		queryParams.Add("project_id", "117")

		httpmock.RegisterMatcherResponderWithQuery(http.MethodGet, fmt.Sprintf("%s%s", usDataEndpoint, exportUrl), queryParams, httpmock.Matcher{}, func(req *http.Request) (*http.Response, error) {
			body := `
			{"event":"test","properties":{"time":1684951135,"$insert_id":"gBDfegagszvqnBsBdDapllybFlqoBarmCEhD","$lib_version":"v0.0.1","$mp_api_endpoint":"api.mixpanel.com","$mp_api_timestamp_ms":1684951135296,"mp_lib":"go","mp_processing_time_ms":1684951136150}}
			{"event":"test_2","properties":{"time":1684951332,"$insert_id":"ydBqfdcrjwuznjkdzygufEkkxkarhCelCrzr","$lib_version":"v0.0.1","$mp_api_endpoint":"api.mixpanel.com","$mp_api_timestamp_ms":1684951332237,"mp_lib":"go","mp_processing_time_ms":1684951332258}}
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewApiClient("token", ServiceAccount(117, "username", "secret"))
		events, err := mp.Export(ctx, parseDate(t, "2023-01-01"), parseDate(t, "2023-01-02"), ExportNoLimit, ExportNoEventFilter, ExportNoWhereFilter)
		require.NoError(t, err)

		require.Len(t, events, 2)
		require.Equal(t, "test", events[0].Name)
		require.Equal(t, "test_2", events[1].Name)
	})

	t.Run("event export with service account", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		queryParams := url.Values{}
		queryParams.Add("from_date", "2023-01-01")
		queryParams.Add("to_date", "2023-01-02")
		queryParams.Add("project_id", "117")

		httpmock.RegisterMatcherResponderWithQuery(http.MethodGet, fmt.Sprintf("%s%s", usDataEndpoint, exportUrl), queryParams, httpmock.Matcher{}, func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})

		mp := NewApiClient("token", ServiceAccount(117, "username", "secret"))
		_, err := mp.Export(ctx, parseDate(t, "2023-01-01"), parseDate(t, "2023-01-02"), ExportNoLimit, ExportNoEventFilter, ExportNoWhereFilter)
		require.NoError(t, err)
	})

	t.Run("event export with no service account", func(t *testing.T) {
		// project_id param can't be send if using no service account for auth

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		queryParams := url.Values{}
		queryParams.Add("from_date", "2023-01-01")
		queryParams.Add("to_date", "2023-01-02")

		httpmock.RegisterMatcherResponderWithQuery(http.MethodGet, fmt.Sprintf("%s%s", usDataEndpoint, exportUrl), queryParams, httpmock.Matcher{}, func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})

		mp := NewApiClient("token")
		_, err := mp.Export(ctx, parseDate(t, "2023-01-01"), parseDate(t, "2023-01-02"), ExportNoLimit, ExportNoEventFilter, ExportNoWhereFilter)
		require.NoError(t, err)
	})
}
