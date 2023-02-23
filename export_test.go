package mixpanel

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func parseDate(t *testing.T, s string) civil.Date {
	d, err := civil.ParseDate(s)
	require.NoError(t, err)
	return d
}

func TestExport(t *testing.T) {
	ctx := context.Background()

	t.Run("event export with service account", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodGet, "https://data.mixpanel.com/api/2.0/export?from_date=2023-01-01&project_id=117&to_date=2023-01-01", func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("")),
			}, nil
		})

		mp := NewClient(117, "token", "api-secret", SetServiceAccount("username", "secret"))
		_, err := mp.Export(ctx, parseDate(t, "2023-01-01"), parseDate(t, "2023-01-02"), ExportNoLimit, ExportNoEventFilter, ExportNoWhereFilter)
		require.NoError(t, err)
	})

	t.Run("event export with no service account", func(t *testing.T) {
		// project_id param can't be send if using no service account for auth

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodGet, "https://data.mixpanel.com/api/2.0/export?from_date=2023-01-01&to_date=2023-01-01", func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("")),
			}, nil
		})

		mp := NewClient(0, "token", "api-secret")
		_, err := mp.Export(ctx, parseDate(t, "2023-01-01"), parseDate(t, "2023-01-02"), ExportNoLimit, ExportNoEventFilter, ExportNoWhereFilter)
		require.NoError(t, err)
	})
}
