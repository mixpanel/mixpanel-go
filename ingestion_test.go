package mixpanel

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestEvent(t *testing.T) {
	t.Run("does not panic with nil properties", func(t *testing.T) {
		mp := NewClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		require.NotNil(t, event.Properties)
	})

	t.Run("event add times correctly", func(t *testing.T) {
		nowTime := time.Now()

		mp := NewClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		event.AddTime(nowTime)

		require.Equal(t, nowTime.UnixMilli(), event.Properties[propertyTime])
	})

	t.Run("insert id set correctly", func(t *testing.T) {
		mp := NewClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		event.AddInsertID("insert-id")

		require.Equal(t, "insert-id", event.Properties[propertyInsertID])
	})

	t.Run("ip sets correctly", func(t *testing.T) {
		ip := net.ParseIP("10.1.1.117")
		require.NotNil(t, ip)

		mp := NewClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		event.AddIP(ip)

		require.Equal(t, ip.String(), event.Properties[propertyIP])
	})

	t.Run("does not panic if ip is nill", func(t *testing.T) {
		mp := NewClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		event.AddIP(nil)

		require.NotContains(t, event.Properties, propertyIP)
	})
}

func TestNewEventFromJson(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		jsonPayload := `
		{
			"properties": {
			  "key": "value"
			},
			"event": "test_event"
		  }
		`
		var payload map[string]any
		err := json.Unmarshal([]byte(jsonPayload), &payload)
		require.NoError(t, err)

		mp := NewClient("token")
		event, err := mp.NewEventFromJson(payload)
		require.NoError(t, err)

		require.Equal(t, "test_event", event.Name)
		require.Equal(t, "value", event.Properties["key"])
	})

	t.Run("event name is missing", func(t *testing.T) {
		jsonPayload := `
		{
			"properties": {
			  "key": "value"
			}
		  }
		`
		var payload map[string]any
		err := json.Unmarshal([]byte(jsonPayload), &payload)
		require.NoError(t, err)

		mp := NewClient("token")
		_, err = mp.NewEventFromJson(payload)
		require.Error(t, err)
	})

	t.Run("event name is missing", func(t *testing.T) {
		jsonPayload := `
		{
			"properties": "not a map",
			"event": "test_event"
		}
		`
		var payload map[string]any
		err := json.Unmarshal([]byte(jsonPayload), &payload)
		require.NoError(t, err)

		mp := NewClient("token")
		_, err = mp.NewEventFromJson(payload)
		require.Error(t, err)
	})
}

func TestTrack(t *testing.T) {
	t.Run("can track an event", func(t *testing.T) {
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
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		require.NoError(t, mp.Track(ctx, events))
	})

	t.Run("can track an event to the eu", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		mp := NewClient("token", EuResidency())
		events := []*Event{
			mp.NewEvent("sample_event", EmptyDistinctID, map[string]any{}),
		}

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", euEndpoint, trackURL), func(req *http.Request) (*http.Response, error) {
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
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		require.NoError(t, mp.Track(ctx, events))
	})

	t.Run("can track multiple event", func(t *testing.T) {
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
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		require.NoError(t, mp.Track(ctx, events))
	})

	t.Run("return error # of events are more than track allows", func(t *testing.T) {
		ctx := context.Background()
		mp := NewClient("token")
		var events []*Event
		for i := 0; i < MaxTrackEvents+1; i++ {
			events = append(events, mp.NewEvent("some event", EmptyDistinctID, map[string]any{}))
		}

		err := mp.Track(ctx, events)
		require.Error(t, err)
	})

	t.Run("track call failed and return error", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		mp := NewClient("token")

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, trackURL), func(req *http.Request) (*http.Response, error) {
			body := `
			{
			  "error": "",
			  "status": 0
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		err := mp.Track(ctx, []*Event{mp.NewEvent("test-event", EmptyDistinctID, map[string]any{})})
		var g VerboseError
		require.ErrorAs(t, err, &g)
	})
}

func TestImport(t *testing.T) {
	ctx := context.Background()

	getValues := func(projectID int, strict bool) url.Values {
		query := url.Values{}
		query.Add("verbose", "1")
		if strict {
			query.Add("strict", "1")
		} else {
			query.Add("strict", "0")
		}
		query.Add("project_id", strconv.Itoa(projectID))
		return query
	}

	t.Run("import successfully", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			contentType := req.Header.Get(contentTypeHeader)
			require.Equal(t, contentType, contentTypeApplicationJson)
			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117), ApiSecret("api-secret"))
		success, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{})
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("content header type set correctly", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			contentType := req.Header.Get(contentTypeHeader)
			require.Equal(t, contentType, contentTypeApplicationJson)
			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117), ApiSecret("api-secret"))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{})
		require.NoError(t, err)
	})

	t.Run("api-secret-auth", func(t *testing.T) {
		apiSecret := "api-secret-auth"

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), getValues(117, false), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get("Authorization")

			require.Equal(t, fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(apiSecret+":"))), authHeader)

			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117), ApiSecret(apiSecret))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{})
		require.NoError(t, err)
	})

	t.Run("api-service-account-auth", func(t *testing.T) {
		userName := "username"
		secret := "secret"

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get("Authorization")

			require.Equal(t, fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(userName+":"+secret))), authHeader)
			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117), ServiceAccount(userName, secret))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{})
		require.NoError(t, err)
	})

	t.Run("api-compression-none", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), getValues(117, false), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get(contentEncodingHeader)
			require.Equal(t, "", authHeader)

			data, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var e []*Event
			require.NoError(t, json.Unmarshal(data, &e))
			require.Len(t, e, 1)

			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Compression: None,
		})
		require.NoError(t, err)
	})

	t.Run("api-compression-gzip", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), getValues(117, false), func(req *http.Request) (*http.Response, error) {
			authHeader := req.Header.Get(contentEncodingHeader)
			require.Equal(t, "gzip", authHeader)

			reader, err := gzip.NewReader(req.Body)
			require.NoError(t, err)

			data, err := io.ReadAll(reader)
			require.NoError(t, err)

			var e []*Event
			require.NoError(t, json.Unmarshal(data, &e))
			require.Len(t, e, 1)

			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Compression: Gzip,
		})
		require.NoError(t, err)
	})

	t.Run("api-strict-enable", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), getValues(117, true), func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()

			require.Equal(t, "1", query.Get("strict"))

			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Strict: true,
		})
		require.NoError(t, err)
	})

	t.Run("api-strict-disable", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), getValues(117, false), func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()

			require.Equal(t, "0", query.Get("strict"))

			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptions{
			Strict: false,
		})
		require.NoError(t, err)
	})

	t.Run("api-project-set-correctly", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), getValues(117, false), func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()

			require.Equal(t, "117", query.Get("project_id"))

			body := `
			{
			  "code": 200,
			  "num_records_imported": 1,
			  "status": 1
			}
			`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})
	})

	t.Run("bad request", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			body := `
			{
				"code": 400,
				"status": "Bad Request",
				"num_records_imported": 1,
				"error": "some data points in the request failed validation",
				"failed_records": [
					{
						"index": 1,
						"field": "event",
						"insert_id": "some-insert-id",
						"message": "'event' must not be missing or blank"
					}
				]
			}
			`

			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
		require.ErrorAs(t, err, &ImportFailedValidationError{})
	})

	t.Run("unauthorized", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			body := `
			{
			  "code": 401,
			  "error":"Unauthorized",
			  "status": 0
			}
			`

			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
		require.ErrorAs(t, err, &ImportGenericError{})
	})

	t.Run("request entity too large", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			body := `
			{
			  "code": 429,
			  "error":"to large",
			  "status": 0
			}
			`

			return &http.Response{
				StatusCode: http.StatusRequestEntityTooLarge,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
		require.ErrorAs(t, err, &ImportGenericError{})
	})

	t.Run("to many requests", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			body := `
			{
			  "code": 429,
			  "error":"Project exceeded rate limits. Please retry the request with exponential backoff.",
			  "status": 0
			}
			`

			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
		require.ErrorAs(t, err, &ImportRateLimitError{})
	})

	t.Run("unknown status code", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, importURL), func(req *http.Request) (*http.Response, error) {
			body := `
			{
			  "code": 418,
			  "error":"I am a teapot",
			  "status": 0
			}
			`

			return &http.Response{
				StatusCode: http.StatusTeapot,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token", ProjectID(117))
		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
		require.Error(t, err)
	})

	t.Run("don't send more events then allowed", func(t *testing.T) {
		ctx := context.Background()
		mp := NewClient("token")
		var events []*Event
		for i := 0; i < MaxImportEvents+1; i++ {
			events = append(events, mp.NewEvent("some event", EmptyDistinctID, map[string]any{}))
		}

		_, err := mp.Import(ctx, events, ImportOptionsRecommend)
		require.Error(t, err)
	})
}

func TestPeopleProperties(t *testing.T) {
	t.Run("nil properties doesn't panic", func(t *testing.T) {
		props := NewPeopleProperties("some-id", nil)
		require.NotNil(t, props)
	})

	t.Run("can set reserved properties", func(t *testing.T) {
		props := NewPeopleProperties("some-id", map[string]any{})
		props.SetReservedProperty(PeopleEmailProperty, "some-email")
		require.Equal(t, "some-email", props.Properties["$email"])
	})

	t.Run("can set ip property", func(t *testing.T) {
		ip := net.ParseIP("10.1.1.117")
		require.NotNil(t, ip)

		props := NewPeopleProperties("some-id", map[string]any{})
		props.SetIp(ip)
		require.Equal(t, ip.String(), props.Properties[string(PeopleGeolocationByIpProperty)])
	})

	t.Run("doesn't set ip if invalid", func(t *testing.T) {
		props := NewPeopleProperties("some-id", map[string]any{})
		props.SetIp(nil)
		require.NotContains(t, props.Properties, string(PeopleGeolocationByIpProperty))
	})
}

func TestPeopleSet(t *testing.T) {
	t.Run("can set one person", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleSetURL), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

			require.Len(t, postBody, 1)

			peopleSet := postBody[0]
			require.Equal(t, "some-id", peopleSet["$distinct_id"])

			set, ok := peopleSet["$set"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "some-value", set["some-key"])

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
			NewPeopleProperties("some-id", map[string]any{
				"some-key": "some-value",
			}),
		}))
	})

	t.Run("can set multiple person", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleSetURL), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))
			require.Len(t, postBody, 2)

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
			NewPeopleProperties("some-id-1", map[string]any{
				"some-key": "some-value-1",
			}),
			NewPeopleProperties("some-id-2", map[string]any{
				"some-key": "some-value-2",
			}),
		}))
	})
}

func TestPeopleSetOnce(t *testing.T) {
	t.Run("can set one person", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleSetOnceURL), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

			require.Len(t, postBody, 1)

			peopleSet := postBody[0]
			require.Equal(t, "some-id", peopleSet["$distinct_id"])

			set, ok := peopleSet["$set_once"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "some-value", set["some-key"])

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleSetOnce(ctx, []*PeopleProperties{
			NewPeopleProperties("some-id", map[string]any{
				"some-key": "some-value",
			}),
		}))
	})
}

func TestPeoplePeopleIncrement(t *testing.T) {
	t.Run("can increment 1 property", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleIncrementUrl), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

			require.Len(t, postBody, 1)

			peopleIncr := postBody[0]
			require.Equal(t, "some-id", peopleIncr["$distinct_id"])

			set, ok := peopleIncr["$add"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, float64(1), set["some-key"])

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleIncrement(ctx, "some-id", map[string]int{
			"some-key": 1,
		}))
	})
}

func TestPeopleAppendListProperty(t *testing.T) {
	t.Run("can add to list", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleAppendToListUrl), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

			require.Len(t, postBody, 1)

			peopleAppend := postBody[0]
			require.Equal(t, "some-id", peopleAppend["$distinct_id"])

			data, ok := peopleAppend["$append"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "some-value", data["list-key"])

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleAppendListProperty(ctx, "some-id", map[string]any{
			"list-key": "some-value",
		}))
	})
}

func TestPeopleRemoveListProperty(t *testing.T) {
	t.Run("can remove from list", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleRemoveFromListUrl), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

			require.Len(t, postBody, 1)

			peopleRemove := postBody[0]
			require.Equal(t, "some-id", peopleRemove["$distinct_id"])

			data, ok := peopleRemove["$remove"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "some-value", data["list-key"])

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleRemoveListProperty(ctx, "some-id", map[string]any{
			"list-key": "some-value",
		}))
	})
}

func TestPeopleDeleteProperty(t *testing.T) {
	t.Run("can delete property", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleDeletePropertyUrl), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

			require.Len(t, postBody, 1)

			unset := postBody[0]
			require.Equal(t, "some-id", unset["$distinct_id"])

			data, ok := unset["$unset"].([]any)
			require.True(t, ok)
			require.Len(t, data, 1)
			require.Contains(t, data, "prop-key")

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleDeleteProperty(ctx, "some-id", []string{"prop-key"}))
	})
}

func TestPeopleDeleteProfile(t *testing.T) {
	t.Run("can delete profile", func(t *testing.T) {
		ctx := context.Background()

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleDeleteProfileUrl), func(req *http.Request) (*http.Response, error) {
			var postBody []map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

			require.Len(t, postBody, 1)

			delete := postBody[0]
			require.Equal(t, "some-id", delete["$distinct_id"])

			data, ok := delete["$distinct_id"].(string)
			require.True(t, ok)
			require.Equal(t, data, "some-id")

			body := `
			1
			`

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})

		mp := NewClient("token")
		require.NoError(t, mp.PeopleDeleteProfile(ctx, "some-id", true))
	})
}
