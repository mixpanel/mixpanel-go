package mixpanel

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/csv"
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
		mp := NewApiClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		require.NotNil(t, event.Properties)
	})

	t.Run("event add times correctly", func(t *testing.T) {
		nowTime := time.Now()

		mp := NewApiClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		event.AddTime(nowTime)

		require.Equal(t, nowTime.UnixMilli(), event.Properties[propertyTime])
	})

	t.Run("insert id set correctly", func(t *testing.T) {
		mp := NewApiClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		event.AddInsertID("insert-id")

		require.Equal(t, "insert-id", event.Properties[propertyInsertID])
	})

	t.Run("ip sets correctly", func(t *testing.T) {
		ip := net.ParseIP("10.1.1.117")
		require.NotNil(t, ip)

		mp := NewApiClient("")
		event := mp.NewEvent("some event", EmptyDistinctID, nil)
		event.AddIP(ip)

		require.Equal(t, ip.String(), event.Properties[propertyIP])
	})

	t.Run("does not panic if ip is nil", func(t *testing.T) {
		mp := NewApiClient("")
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

		mp := NewApiClient("token")
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

		mp := NewApiClient("token")
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

		mp := NewApiClient("token")
		_, err = mp.NewEventFromJson(payload)
		require.Error(t, err)
	})
}

func TestTrack(t *testing.T) {
	setupHttpEndpointTest := func(t *testing.T, client *ApiClient, testPayload func([]*Event), httpResponse *http.Response) {
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)

		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, trackURL), func(req *http.Request) (*http.Response, error) {
			require.Equal(t, req.Header.Get("content-type"), "application/json")
			require.Equal(t, req.Header.Get("accept"), "text/plain")
			require.Equal(t, "1", req.URL.Query().Get("verbose"))

			var r []*Event
			require.NoError(t, json.NewDecoder(req.Body).Decode(&r))
			testPayload(r)

			return httpResponse, nil
		})
	}

	trackSuccess := func() *http.Response {
		body := `
			{
			  "error": "",
			  "status": 1
			}
			`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
		}
	}

	t.Run("can track an event", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		events := []*Event{
			mp.NewEvent("sample_event", EmptyDistinctID, map[string]any{}),
		}
		setupHttpEndpointTest(t, mp, func(r []*Event) {
			require.Len(t, r, 1)
			require.ElementsMatch(t, events, r)
		}, trackSuccess())

		require.NoError(t, mp.Track(ctx, events))
	})

	t.Run("eu data residency works correctly", func(t *testing.T) {
		ctx := context.Background()
		euMixpanel := NewApiClient("token", EuResidency())

		events := []*Event{
			euMixpanel.NewEvent("sample_event", EmptyDistinctID, map[string]any{}),
		}
		setupHttpEndpointTest(t, euMixpanel, func(r []*Event) {
			require.Len(t, r, 1)
			require.ElementsMatch(t, events, r)
		}, trackSuccess())

		require.NoError(t, euMixpanel.Track(ctx, events))

		usMixpanel := NewApiClient("token")
		require.Error(t, usMixpanel.Track(ctx, events))
	})

	t.Run("track multiple events successfully", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		events := []*Event{
			mp.NewEvent("sample_event_1", EmptyDistinctID, map[string]any{}),
			mp.NewEvent("sample_event_2", EmptyDistinctID, map[string]any{}),
			mp.NewEvent("sample_event_3", EmptyDistinctID, map[string]any{}),
		}

		setupHttpEndpointTest(t, mp, func(r []*Event) {
			require.Len(t, r, 3)
			require.ElementsMatch(t, events, r)
		}, trackSuccess())

		require.NoError(t, mp.Track(ctx, events))
	})

	t.Run("return error # of events are more than track allows", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")
		var events []*Event
		for i := 0; i < MaxTrackEvents+1; i++ {
			events = append(events, mp.NewEvent("some event", EmptyDistinctID, map[string]any{}))
		}

		err := mp.Track(ctx, events)
		require.Error(t, err)
	})

	t.Run("track call failed and return error", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		events := []*Event{
			mp.NewEvent("sample_event", EmptyDistinctID, map[string]any{}),
		}
		setupHttpEndpointTest(t, mp, func(r []*Event) {
			require.Len(t, r, 1)
			require.ElementsMatch(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`
			{
				"error": "some error occurred",
				"status": 0
			  }
			`)),
		})

		require.Error(t, mp.Track(ctx, events))
	})
}

func TestImport(t *testing.T) {
	setupHttpEndpointTest := func(t *testing.T, client *ApiClient, queryValues url.Values, testPayload func([]*Event), httpResponse *http.Response) {
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, importURL), queryValues, func(req *http.Request) (*http.Response, error) {
			auth := req.Header.Get("authorization")
			if client.serviceAccount != nil {
				require.Equal(t, auth, "Basic "+base64.StdEncoding.EncodeToString([]byte(client.serviceAccount.Username+":"+client.serviceAccount.Secret)))
			} else if client.apiSecret != "" {
				require.Equal(t, auth, "Basic "+base64.StdEncoding.EncodeToString([]byte(client.apiSecret+":")))
			} else {
				require.Equal(t, auth, "Basic "+base64.StdEncoding.EncodeToString([]byte(client.token+":")))
			}

			compress := req.Header.Get("content-encoding")
			reader := req.Body
			if compress == "gzip" {
				require.Equal(t, req.Header.Get("Content-Encoding"), "gzip")
				var err error
				reader, err = gzip.NewReader(req.Body)
				require.NoError(t, err)
			} else {
				require.Equal(t, req.Header.Get("Content-Encoding"), "")
			}

			var r []*Event
			require.NoError(t, json.NewDecoder(reader).Decode(&r))
			testPayload(r)

			return httpResponse, nil
		})
	}

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
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))

		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}
		setupHttpEndpointTest(t, mp, getValues(117, ImportOptionsRecommend.Strict), func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code": 200,"num_records_imported": 1,"status": 1}`)),
		})

		success, err := mp.Import(ctx, events, ImportOptionsRecommend)
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("api-secret", func(t *testing.T) {
		query := url.Values{}
		query.Add("verbose", "1")
		query.Add("strict", "1")

		ctx := context.Background()
		mp := NewApiClient("token", ApiSecret("some-secret"))
		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}

		setupHttpEndpointTest(t, mp, query, func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code": 200,"num_records_imported": 1,"status": 1}`)),
		})

		success, err := mp.Import(ctx, events, ImportOptionsRecommend)
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("api-token", func(t *testing.T) {
		query := url.Values{}
		query.Add("verbose", "1")
		query.Add("strict", "1")

		ctx := context.Background()
		mp := NewApiClient("token")
		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}

		setupHttpEndpointTest(t, mp, query, func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code": 200,"num_records_imported": 1,"status": 1}`)),
		})

		success, err := mp.Import(ctx, events, ImportOptionsRecommend)
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("can import gzip if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))
		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}

		setupHttpEndpointTest(t, mp, getValues(117, ImportOptionsRecommend.Strict), func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code": 200,"num_records_imported": 1,"status": 1}`)),
		})

		success, err := mp.Import(ctx, events, ImportOptions{
			Strict:      true,
			Compression: Gzip,
		})
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("can import non gzip data if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))
		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}

		setupHttpEndpointTest(t, mp, getValues(117, ImportOptionsRecommend.Strict), func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code": 200,"num_records_imported": 1,"status": 1}`)),
		})

		success, err := mp.Import(ctx, events, ImportOptions{
			Strict:      true,
			Compression: None,
		})
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("can enable strict mode if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))
		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}

		setupHttpEndpointTest(t, mp, getValues(117, true), func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code": 200,"num_records_imported": 1,"status": 1}`)),
		})

		success, err := mp.Import(ctx, events, ImportOptions{
			Strict:      true,
			Compression: Gzip,
		})
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("can disable strict mode if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))
		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}

		setupHttpEndpointTest(t, mp, getValues(117, false), func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code": 200,"num_records_imported": 1,"status": 1}`)),
		})

		success, err := mp.Import(ctx, events, ImportOptions{
			Strict:      false,
			Compression: Gzip,
		})
		require.NoError(t, err)

		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("bad request", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))
		setupHttpEndpointTest(t, mp, getValues(117, ImportOptionsRecommend.Strict), func(r []*Event) {}, &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(strings.NewReader(`
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
			`)),
		})

		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
		validationError := &ImportFailedValidationError{}
		require.ErrorAs(t, err, validationError)
		require.Equal(t, 1, validationError.NumRecordsImported)
		require.Equal(t, 1, len(validationError.FailedImportRecords))
		require.Equal(t, "some-insert-id", validationError.FailedImportRecords[0].InsertID)
		require.Equal(t, "event", validationError.FailedImportRecords[0].Field)
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))

		setupHttpEndpointTest(t, mp, getValues(117, ImportOptionsRecommend.Strict), func(r []*Event) {}, &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body: io.NopCloser(strings.NewReader(`
			{
				"code": 429,
				"error":"rate limit exceeded",
				"status": 0
			}
			`)),
		})

		_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
		rateLimitError := &ImportRateLimitError{}
		require.ErrorAs(t, err, rateLimitError)
	})

	t.Run("test know status code errors", func(t *testing.T) {
		tests := []struct {
			httpStatusCode int
		}{
			{
				httpStatusCode: http.StatusUnauthorized,
			},
			{
				httpStatusCode: http.StatusRequestEntityTooLarge,
			},
		}

		for _, test := range tests {
			t.Run(fmt.Sprintf("http status code %d", test.httpStatusCode), func(t *testing.T) {
				ctx := context.Background()
				mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))
				setupHttpEndpointTest(t, mp, getValues(117, ImportOptionsRecommend.Strict), func(r []*Event) {}, &http.Response{
					StatusCode: test.httpStatusCode,
					Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`
					{
						"code": %d,
						"error":"Unauthorized",
						"status": 0
					  }
					`, test.httpStatusCode))),
				})

				_, err := mp.Import(ctx, []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}, ImportOptionsRecommend)
				genericError := &ImportGenericError{}
				require.ErrorAs(t, err, genericError)
				require.Equal(t, test.httpStatusCode, genericError.Code)
			})
		}
	})

	t.Run("unknown status code", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token", ServiceAccount(117, "user-name", "secret"))
		events := []*Event{mp.NewEvent("import-event", EmptyDistinctID, map[string]any{})}

		setupHttpEndpointTest(t, mp, getValues(117, false), func(r []*Event) {
			require.Equal(t, events, r)
		}, &http.Response{
			StatusCode: http.StatusTeapot,
			Body:       io.NopCloser(strings.NewReader("i'm a teapot")),
		})

		_, err := mp.Import(ctx, events, ImportOptions{
			Strict:      false,
			Compression: Gzip,
		})
		require.Error(t, err)
	})

	t.Run("don't send more events then allowed", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")
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

	t.Run("can use server ip", func(t *testing.T) {
		props := NewPeopleProperties("some-id", map[string]any{})
		props.SetIp(nil, UseRequestIp())
		require.Equal(t, true, props.UseRequestIp)
	})

	t.Run("doesn't set ip if invalid", func(t *testing.T) {
		props := NewPeopleProperties("some-id", map[string]any{})
		props.SetIp(nil)
		require.NotContains(t, props.Properties, string(PeopleGeolocationByIpProperty))
	})

	t.Run("0 if no ip is provided", func(t *testing.T) {
		props := NewPeopleProperties("some-id", map[string]any{})
		require.Equal(t, "0", props.shouldGeoLookupIp())
	})

	t.Run("ip is set if ip is provided", func(t *testing.T) {
		ip := net.ParseIP("10.1.1.117")
		require.NotNil(t, ip)

		props := NewPeopleProperties("some-id", map[string]any{
			string(PeopleGeolocationByIpProperty): ip.String(),
		})
		require.Equal(t, "10.1.1.117", props.shouldGeoLookupIp())
	})

	t.Run("0 if value if ip is not a string", func(t *testing.T) {
		ip := net.ParseIP("10.1.1.117")
		require.NotNil(t, ip)

		props := NewPeopleProperties("some-id", map[string]any{
			string(PeopleGeolocationByIpProperty): ip,
		})
		require.Equal(t, "0", props.shouldGeoLookupIp())
	})
}

func setupPeopleAndGroupsEndpoint(t *testing.T, client *ApiClient, endpoint string, testPayload func(body io.Reader), httpResponse *http.Response) {
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)

	httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, endpoint), func(req *http.Request) (*http.Response, error) {
		require.Equal(t, req.Header.Get("content-type"), "application/json")
		require.Equal(t, req.Header.Get("accept"), "text/plain")

		testPayload(req.Body)

		return httpResponse, nil
	})
}

var makePeopleAndGroupResponse = func(code string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(code)),
	}
}

var peopleAndGroupSuccess = func() *http.Response {
	return makePeopleAndGroupResponse("1")
}

func TestPeopleSet(t *testing.T) {
	t.Run("can set one person", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		people := NewPeopleProperties("some-id", map[string]any{
			"some-key": "some-value",
		})

		setupPeopleAndGroupsEndpoint(t, mp, peopleSetURL, func(body io.Reader) {
			payload := []*peopleSetPayload{}
			require.NoError(t, json.NewDecoder(body).Decode(&payload))

			require.Len(t, payload, 1)
			require.Equal(t, people.DistinctID, payload[0].DistinctID)
			require.Equal(t, "0", payload[0].IP)

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
			people,
		}))
	})

	t.Run("track ip if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		people := NewPeopleProperties("some-id", map[string]any{
			"some-key":                            "some-value",
			string(PeopleGeolocationByIpProperty): "127.0.0.1",
		})

		setupPeopleAndGroupsEndpoint(t, mp, peopleSetURL, func(body io.Reader) {
			payload := []*peopleSetPayload{}
			require.NoError(t, json.NewDecoder(body).Decode(&payload))

			require.Len(t, payload, 1)
			require.Equal(t, people.DistinctID, payload[0].DistinctID)
			require.Equal(t, "127.0.0.1", payload[0].IP)

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
			people,
		}))
	})

	t.Run("use request ip if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		people := NewPeopleProperties("some-id", map[string]any{
			"some-key": "some-value",
		})
		people.SetIp(nil, UseRequestIp())

		setupPeopleAndGroupsEndpoint(t, mp, peopleSetURL, func(body io.Reader) {
			stringBody, err := io.ReadAll(body)
			require.NoError(t, err)

			payload := []*peopleSetPayload{}
			require.NoError(t, json.NewDecoder(strings.NewReader(string(stringBody))).Decode(&payload))

			require.Len(t, payload, 1)
			require.Equal(t, people.DistinctID, payload[0].DistinctID)
			require.False(t, strings.Contains(string(stringBody), "$ip"))

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
			people,
		}))
	})

	t.Run("can set multiple people", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		person1 := NewPeopleProperties("some-id-1", map[string]any{
			"some-key": "some-value",
		})
		person2 := NewPeopleProperties("some-id-2", map[string]any{
			"some-key": "some-value",
		})

		setupPeopleAndGroupsEndpoint(t, mp, peopleSetURL, func(body io.Reader) {
			payload := []*peopleSetPayload{}
			require.NoError(t, json.NewDecoder(body).Decode(&payload))

			require.Len(t, payload, 2)
			require.Equal(t, mp.token, payload[0].Token)
			require.Equal(t, person1.DistinctID, payload[0].DistinctID)
			require.Equal(t, mp.token, payload[1].Token)
			require.Equal(t, person2.DistinctID, payload[1].DistinctID)

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
			person1,
			person2,
		}))
	})

	t.Run("can not go above the limit", func(t *testing.T) {
		ctx := context.Background()

		mp := NewApiClient("token")
		var people []*PeopleProperties
		for i := 0; i < MaxPeopleEvents+1; i++ {
			people = append(people, NewPeopleProperties("some-id", map[string]any{}))
		}

		require.Error(t, mp.PeopleSet(ctx, people))
	})
}

func TestPeopleSetOnce(t *testing.T) {
	t.Run("can set one", func(t *testing.T) {
		ctx := context.Background()

		mp := NewApiClient("token")
		person1 := NewPeopleProperties("some-id-1", map[string]any{
			"some-key": "some-value",
		})
		setupPeopleAndGroupsEndpoint(t, mp, peopleSetOnceURL, func(body io.Reader) {
			payload := []*peopleSetOncePayload{}
			require.NoError(t, json.NewDecoder(body).Decode(&payload))

			require.Len(t, payload, 1)
			require.Equal(t, mp.token, payload[0].Token)
			require.Equal(t, person1.DistinctID, payload[0].DistinctID)
			require.Equal(t, "0", payload[0].IP)

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSetOnce(ctx, []*PeopleProperties{person1}))
	})

	t.Run("can set one", func(t *testing.T) {
		ctx := context.Background()

		mp := NewApiClient("token")
		person1 := NewPeopleProperties("some-id-1", map[string]any{
			"some-key":                            "some-value",
			string(PeopleGeolocationByIpProperty): "127.0.0.1",
		})
		setupPeopleAndGroupsEndpoint(t, mp, peopleSetOnceURL, func(body io.Reader) {
			payload := []*peopleSetOncePayload{}
			require.NoError(t, json.NewDecoder(body).Decode(&payload))

			require.Len(t, payload, 1)
			require.Equal(t, mp.token, payload[0].Token)
			require.Equal(t, person1.DistinctID, payload[0].DistinctID)
			require.Equal(t, "127.0.0.1", payload[0].IP)

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSetOnce(ctx, []*PeopleProperties{person1}))
	})

	t.Run("track ip if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		people := NewPeopleProperties("some-id", map[string]any{
			"some-key":                            "some-value",
			string(PeopleGeolocationByIpProperty): "127.0.0.1",
		})

		setupPeopleAndGroupsEndpoint(t, mp, peopleSetOnceURL, func(body io.Reader) {
			payload := []*peopleSetOncePayload{}
			require.NoError(t, json.NewDecoder(body).Decode(&payload))

			require.Len(t, payload, 1)
			require.Equal(t, people.DistinctID, payload[0].DistinctID)
			require.Equal(t, "127.0.0.1", payload[0].IP)

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSetOnce(ctx, []*PeopleProperties{
			people,
		}))
	})

	t.Run("use request ip if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("token")

		people := NewPeopleProperties("some-id", map[string]any{
			"some-key": "some-value",
		})
		people.SetIp(nil, UseRequestIp())

		setupPeopleAndGroupsEndpoint(t, mp, peopleSetOnceURL, func(body io.Reader) {
			stringBody, err := io.ReadAll(body)
			require.NoError(t, err)

			payload := []*peopleSetOncePayload{}
			require.NoError(t, json.NewDecoder(strings.NewReader(string(stringBody))).Decode(&payload))

			require.Len(t, payload, 1)
			require.Equal(t, people.DistinctID, payload[0].DistinctID)
			require.False(t, strings.Contains(string(stringBody), "$ip"))

		}, peopleAndGroupSuccess())

		require.NoError(t, mp.PeopleSetOnce(ctx, []*PeopleProperties{
			people,
		}))
	})

	t.Run("can not go above the limit", func(t *testing.T) {
		ctx := context.Background()

		mp := NewApiClient("token")
		var people []*PeopleProperties
		for i := 0; i < MaxPeopleEvents+1; i++ {
			people = append(people, NewPeopleProperties("some-id", map[string]any{}))
		}

		require.Error(t, mp.PeopleSetOnce(ctx, people))
	})
}

func TestPeopleUnion(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, peopleUnionToListUrl, func(body io.Reader) {
		arrayPayload := []*peopleUnionPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "some-id", payload.DistinctID)
		require.Equal(t, "some-value", payload.Union["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.PeopleUnionProperty(ctx, "some-id", map[string]any{
		"some-prop": "some-value",
	}))
}

func TestPeoplePeopleIncrement(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, peopleIncrementUrl, func(body io.Reader) {
		arrayPayload := []*peopleNumericalAddPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "some-id", payload.DistinctID)
		require.Equal(t, 1, payload.Add["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.PeopleIncrement(ctx, "some-id", map[string]int{
		"some-prop": 1,
	}))
}

func TestPeopleAppendListProperty(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, peopleAppendToListUrl, func(body io.Reader) {
		arrayPayload := []*peopleAppendListPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "some-id", payload.DistinctID)
		require.Equal(t, "some-value", payload.Append["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.PeopleAppendListProperty(ctx, "some-id", map[string]any{
		"some-prop": "some-value",
	}))
}

func TestPeopleRemoveListProperty(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, peopleRemoveFromListUrl, func(body io.Reader) {
		arrayPayload := []*peopleListRemovePayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "some-id", payload.DistinctID)
		require.Equal(t, "some-value", payload.Remove["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.PeopleRemoveListProperty(ctx, "some-id", map[string]any{
		"some-prop": "some-value",
	}))
}

func TestPeopleDeleteProperty(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, peopleDeletePropertyUrl, func(body io.Reader) {
		arrayPayload := []*peopleDeletePropertyPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "some-id", payload.DistinctID)
		require.Equal(t, []string{"some-value"}, payload.Unset)

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.PeopleDeleteProperty(ctx, "some-id", []string{"some-value"}))
}

func TestPeopleDeleteProfile(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, peopleDeleteProfileUrl, func(body io.Reader) {
		arrayPayload := []*peopleDeleteProfilePayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "some-id", payload.DistinctID)
		require.Equal(t, "true", payload.IgnoreAlias)

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.PeopleDeleteProfile(ctx, "some-id", true))
}

func TestGroupSetProperty(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, groupSetUrl, func(body io.Reader) {
		arrayPayload := []*groupSetPropertyPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "group-key", payload.GroupKey)
		require.Equal(t, "group-id", payload.GroupId)
		require.Equal(t, "some-value", payload.Set["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.GroupSet(ctx, "group-key", "group-id", map[string]any{
		"some-prop": "some-value",
	}))
}

func TestGroupSetOnce(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, groupsSetOnceUrl, func(body io.Reader) {
		arrayPayload := []*groupSetOncePropertyPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "group-key", payload.GroupKey)
		require.Equal(t, "group-id", payload.GroupId)
		require.Equal(t, "some-value", payload.SetOnce["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.GroupSetOnce(ctx, "group-key", "group-id", map[string]any{
		"some-prop": "some-value",
	}))
}

func TestGroupDeleteProperty(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, groupsDeletePropertyUrl, func(body io.Reader) {
		arrayPayload := []*groupDeletePropertyPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "group-key", payload.GroupKey)
		require.Equal(t, "group-id", payload.GroupId)
		require.Equal(t, []string{"some-value"}, payload.Unset)

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.GroupDeleteProperty(ctx, "group-key", "group-id", []string{"some-value"}))
}

func TestGroupRemoveListProperty(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, groupsRemoveFromListPropertyUrl, func(body io.Reader) {
		arrayPayload := []*groupRemoveListPropertyPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "group-key", payload.GroupKey)
		require.Equal(t, "group-id", payload.GroupId)
		require.Equal(t, "some-value", payload.Remove["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.GroupRemoveListProperty(ctx, "group-key", "group-id", map[string]any{
		"some-prop": "some-value",
	}))
}

func TestGroupUnionListProperty(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, groupsUnionListPropertyUrl, func(body io.Reader) {
		arrayPayload := []*groupUnionListPropertyPayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "group-key", payload.GroupKey)
		require.Equal(t, "group-id", payload.GroupId)
		require.Equal(t, "some-value", payload.Union["some-prop"])

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.GroupUnionListProperty(ctx, "group-key", "group-id", map[string]any{
		"some-prop": "some-value",
	}))
}

func TestGroupDelete(t *testing.T) {
	ctx := context.Background()

	mp := NewApiClient("token")
	setupPeopleAndGroupsEndpoint(t, mp, groupsDeleteGroupUrl, func(body io.Reader) {
		arrayPayload := []*groupDeletePayload{}
		require.NoError(t, json.NewDecoder(body).Decode(&arrayPayload))

		payload := arrayPayload[0]
		require.Equal(t, mp.token, payload.Token)
		require.Equal(t, "group-key", payload.GroupKey)
		require.Equal(t, "group-id", payload.GroupId)

	}, peopleAndGroupSuccess())

	require.NoError(t, mp.GroupDelete(ctx, "group-key", "group-id"))
}

func TestLookupTableReplace(t *testing.T) {
	setupHttpEndpointTest := func(t *testing.T, client *ApiClient, lookupTableID string, testPayload func([][]string), httpResponse *http.Response) {
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)

		httpmock.RegisterResponder(http.MethodPut, fmt.Sprintf("%s%s/%s", client.apiEndpoint, lookupTableReplaceUrl, lookupTableID), func(req *http.Request) (*http.Response, error) {
			require.Equal(t, req.Header.Get("content-type"), "text/csv")
			require.Equal(t, req.Header.Get("accept"), "application/json")

			csvReader := csv.NewReader(req.Body)
			r, err := csvReader.ReadAll()
			require.NoError(t, err)
			testPayload(r)

			return httpResponse, nil
		})
	}

	lookupTableID := "lookupTableID"
	table := [][]string{
		{
			"header 1", "header 2",
		},
		{
			"row_1_col_1", "row_1_col2",
		},
		{
			"row_2_col_1", "row_2_col2",
		},
	}

	replaceSuccess := func() *http.Response {
		body := `
			{
			  "code": 200,
			  "status": "OK"
			}
			`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
		}
	}

	t.Run("can replace a lookup table", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("", ServiceAccount(0, "usernamer", "secret"))

		setupHttpEndpointTest(t, mp, lookupTableID, func(r [][]string) {
			require.Len(t, r, 3)
			require.ElementsMatch(t, table, r)
		}, replaceSuccess())

		require.NoError(t, mp.LookupTableReplace(ctx, lookupTableID, table))
	})

	t.Run("eu data residency works correctly", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("", ServiceAccount(0, "usernamer", "secret"), EuResidency())

		setupHttpEndpointTest(t, mp, lookupTableID, func(r [][]string) {
			require.Len(t, r, 3)
			require.ElementsMatch(t, table, r)
		}, replaceSuccess())

		require.NoError(t, mp.LookupTableReplace(ctx, lookupTableID, table))
	})

	t.Run("return error when service account is not provided", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("")

		setupHttpEndpointTest(t, mp, lookupTableID, func(r [][]string) {
			require.Len(t, r, 3)
			require.ElementsMatch(t, table, r)
		}, replaceSuccess())

		require.Error(t, mp.LookupTableReplace(ctx, lookupTableID, table))
	})

	t.Run("replace call failed and return error", func(t *testing.T) {
		ctx := context.Background()
		mp := NewApiClient("", ServiceAccount(0, "usernamer", "secret"))

		setupHttpEndpointTest(t, mp, lookupTableID, func(r [][]string) {
			require.Len(t, r, 3)
			require.ElementsMatch(t, table, r)
		}, &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(strings.NewReader(`
			{
				"error": "some error occurred",
				"status": 0
			  }
			`)),
		})

		require.Error(t, mp.LookupTableReplace(ctx, lookupTableID, table))
	})
}
