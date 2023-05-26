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
	setupHttpEndpointTest := func(t *testing.T, client *Mixpanel, testPayload func([]*Event), httpResponse *http.Response) {
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
		mp := NewClient("token")

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
		euMixpanel := NewClient("token", EuResidency())

		events := []*Event{
			euMixpanel.NewEvent("sample_event", EmptyDistinctID, map[string]any{}),
		}
		setupHttpEndpointTest(t, euMixpanel, func(r []*Event) {
			require.Len(t, r, 1)
			require.ElementsMatch(t, events, r)
		}, trackSuccess())

		require.NoError(t, euMixpanel.Track(ctx, events))

		usMixpanel := NewClient("token")
		require.Error(t, usMixpanel.Track(ctx, events))
	})

	t.Run("track multiple events successfully", func(t *testing.T) {
		ctx := context.Background()
		mp := NewClient("token")

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
		mp := NewClient("token")

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
	setupHttpEndpointTest := func(t *testing.T, client *Mixpanel, queryValues url.Values, testPayload func([]*Event), httpResponse *http.Response) {
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)

		httpmock.RegisterResponderWithQuery(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, importURL), queryValues, func(req *http.Request) (*http.Response, error) {
			auth := req.Header.Get("authorization")
			if client.serviceAccount != nil {
				require.Equal(t, auth, "Basic "+base64.StdEncoding.EncodeToString([]byte(client.serviceAccount.Username+":"+client.serviceAccount.Secret)))
			} else {
				require.Equal(t, auth, "Basic "+base64.StdEncoding.EncodeToString([]byte(client.apiSecret+":")))
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
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))

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

	t.Run("api-secret-auth", func(t *testing.T) {
		ctx := context.Background()
		mp := NewClient("token", ProjectID(117), ApiSecret("api-secret"))
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

	t.Run("can import gzip if requested", func(t *testing.T) {
		ctx := context.Background()
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))
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
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))
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
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))
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
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))
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
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))
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
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))

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
				mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))
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
		mp := NewClient("token", ProjectID(117), ServiceAccount("user-name", "secret"))
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
		mp := NewClient("token")
		var events []*Event
		for i := 0; i < MaxImportEvents+1; i++ {
			events = append(events, mp.NewEvent("some event", EmptyDistinctID, map[string]any{}))
		}

		_, err := mp.Import(ctx, events, ImportOptionsRecommend)
		require.Error(t, err)
	})
}

// func TestPeopleProperties(t *testing.T) {
// 	t.Run("nil properties doesn't panic", func(t *testing.T) {
// 		props := NewPeopleProperties("some-id", nil)
// 		require.NotNil(t, props)
// 	})

// 	t.Run("can set reserved properties", func(t *testing.T) {
// 		props := NewPeopleProperties("some-id", map[string]any{})
// 		props.SetReservedProperty(PeopleEmailProperty, "some-email")
// 		require.Equal(t, "some-email", props.Properties["$email"])
// 	})

// 	t.Run("can set ip property", func(t *testing.T) {
// 		ip := net.ParseIP("10.1.1.117")
// 		require.NotNil(t, ip)

// 		props := NewPeopleProperties("some-id", map[string]any{})
// 		props.SetIp(ip)
// 		require.Equal(t, ip.String(), props.Properties[string(PeopleGeolocationByIpProperty)])
// 	})

// 	t.Run("doesn't set ip if invalid", func(t *testing.T) {
// 		props := NewPeopleProperties("some-id", map[string]any{})
// 		props.SetIp(nil)
// 		require.NotContains(t, props.Properties, string(PeopleGeolocationByIpProperty))
// 	})
// }

// func TestPeopleSet(t *testing.T) {
// 	t.Run("can set one person", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleSetURL), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			peopleSet := postBody[0]
// 			require.Equal(t, "some-id", peopleSet["$distinct_id"])

// 			set, ok := peopleSet["$set"].(map[string]any)
// 			require.True(t, ok)
// 			require.Equal(t, "some-value", set["some-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
// 			NewPeopleProperties("some-id", map[string]any{
// 				"some-key": "some-value",
// 			}),
// 		}))
// 	})

// 	t.Run("can set multiple person", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleSetURL), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))
// 			require.Len(t, postBody, 2)

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{
// 			NewPeopleProperties("some-id-1", map[string]any{
// 				"some-key": "some-value-1",
// 			}),
// 			NewPeopleProperties("some-id-2", map[string]any{
// 				"some-key": "some-value-2",
// 			}),
// 		}))
// 	})

// 	t.Run("can not go above the limit", func(t *testing.T) {
// 		ctx := context.Background()

// 		mp := NewClient("token")
// 		var people []*PeopleProperties
// 		for i := 0; i < MaxPeopleEvents+1; i++ {
// 			people = append(people, NewPeopleProperties("some-id", map[string]any{}))
// 		}

// 		require.Error(t, mp.PeopleSet(ctx, people))
// 	})
// }

// func TestPeopleSetOnce(t *testing.T) {
// 	t.Run("can set one person", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleSetOnceURL), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			peopleSet := postBody[0]
// 			require.Equal(t, "some-id", peopleSet["$distinct_id"])

// 			set, ok := peopleSet["$set_once"].(map[string]any)
// 			require.True(t, ok)
// 			require.Equal(t, "some-value", set["some-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleSetOnce(ctx, []*PeopleProperties{
// 			NewPeopleProperties("some-id", map[string]any{
// 				"some-key": "some-value",
// 			}),
// 		}))
// 	})

// 	t.Run("can not go above the limit", func(t *testing.T) {
// 		ctx := context.Background()

// 		mp := NewClient("token")
// 		var people []*PeopleProperties
// 		for i := 0; i < MaxPeopleEvents+1; i++ {
// 			people = append(people, NewPeopleProperties("some-id", map[string]any{}))
// 		}

// 		require.Error(t, mp.PeopleSetOnce(ctx, people))
// 	})
// }

// func TestPeopleUnion(t *testing.T) {
// 	t.Run("can union a property", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleUnionToListUrl), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			peopleUnion := postBody[0]
// 			require.Equal(t, "some-id", peopleUnion["$distinct_id"])

// 			union, ok := peopleUnion["$union"].(map[string]any)
// 			require.True(t, ok)
// 			require.Equal(t, "some-value", union["some-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleUnionProperty(ctx, "some-id", map[string]any{
// 			"some-key": "some-value",
// 		}))
// 	})
// }

// func TestPeoplePeopleIncrement(t *testing.T) {
// 	t.Run("can increment 1 property", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleIncrementUrl), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			peopleIncr := postBody[0]
// 			require.Equal(t, "some-id", peopleIncr["$distinct_id"])

// 			set, ok := peopleIncr["$add"].(map[string]any)
// 			require.True(t, ok)
// 			require.Equal(t, float64(1), set["some-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleIncrement(ctx, "some-id", map[string]int{
// 			"some-key": 1,
// 		}))
// 	})
// }

// func TestPeopleAppendListProperty(t *testing.T) {
// 	t.Run("can add to list", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleAppendToListUrl), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			peopleAppend := postBody[0]
// 			require.Equal(t, "some-id", peopleAppend["$distinct_id"])

// 			data, ok := peopleAppend["$append"].(map[string]any)
// 			require.True(t, ok)
// 			require.Equal(t, "some-value", data["list-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleAppendListProperty(ctx, "some-id", map[string]any{
// 			"list-key": "some-value",
// 		}))
// 	})
// }

// func TestPeopleRemoveListProperty(t *testing.T) {
// 	t.Run("can remove from list", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleRemoveFromListUrl), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			peopleRemove := postBody[0]
// 			require.Equal(t, "some-id", peopleRemove["$distinct_id"])

// 			data, ok := peopleRemove["$remove"].(map[string]any)
// 			require.True(t, ok)
// 			require.Equal(t, "some-value", data["list-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleRemoveListProperty(ctx, "some-id", map[string]any{
// 			"list-key": "some-value",
// 		}))
// 	})
// }

// func TestPeopleDeleteProperty(t *testing.T) {
// 	t.Run("can delete property", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleDeletePropertyUrl), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			unset := postBody[0]
// 			require.Equal(t, "some-id", unset["$distinct_id"])

// 			data, ok := unset["$unset"].([]any)
// 			require.True(t, ok)
// 			require.Len(t, data, 1)
// 			require.Contains(t, data, "prop-key")

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleDeleteProperty(ctx, "some-id", []string{"prop-key"}))
// 	})
// }

// func TestPeopleDeleteProfile(t *testing.T) {
// 	t.Run("can delete profile", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, peopleDeleteProfileUrl), func(req *http.Request) (*http.Response, error) {
// 			var postBody []map[string]any
// 			require.NoError(t, json.NewDecoder(req.Body).Decode(&postBody))

// 			require.Len(t, postBody, 1)

// 			delete := postBody[0]
// 			require.Equal(t, "some-id", delete["$distinct_id"])

// 			data, ok := delete["$distinct_id"].(string)
// 			require.True(t, ok)
// 			require.Equal(t, data, "some-id")

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.PeopleDeleteProfile(ctx, "some-id", true))
// 	})
// }

// func TestGroupSetProperty(t *testing.T) {
// 	t.Run("can set group property", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, groupSetUrl), func(req *http.Request) (*http.Response, error) {
// 			require.NoError(t, req.ParseForm())
// 			data := req.Form.Get("data")

// 			var postBody []groupSetPropertyPayload
// 			require.NoError(t, json.Unmarshal([]byte(data), &postBody))
// 			require.Len(t, postBody, 1)

// 			groupSet := postBody[0]
// 			require.Equal(t, "token", groupSet.Token)
// 			require.Equal(t, "group-key", groupSet.GroupKey)
// 			require.Equal(t, "group-id", groupSet.GroupId)
// 			require.Equal(t, "some-prop-value", groupSet.Set["some-prop-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.GroupSet(ctx, "group-key", "group-id", map[string]any{
// 			"some-prop-key": "some-prop-value",
// 		}))
// 	})
// }

// func TestGroupSetOnce(t *testing.T) {
// 	t.Run("can set group property once", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, groupsSetOnceUrl), func(req *http.Request) (*http.Response, error) {
// 			require.NoError(t, req.ParseForm())
// 			data := req.Form.Get("data")

// 			var postBody []groupSetOncePropertyPayload
// 			require.NoError(t, json.Unmarshal([]byte(data), &postBody))
// 			require.Len(t, postBody, 1)

// 			groupUpdate := postBody[0]
// 			require.Equal(t, "token", groupUpdate.Token)
// 			require.Equal(t, "group-key", groupUpdate.GroupKey)
// 			require.Equal(t, "group-id", groupUpdate.GroupId)
// 			require.Equal(t, "some-prop-value", groupUpdate.SetOnce["some-prop-key"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.GroupSetOnce(ctx, "group-key", "group-id", map[string]any{
// 			"some-prop-key": "some-prop-value",
// 		}))
// 	})
// }

// func TestGroupDeleteProperty(t *testing.T) {
// 	t.Run("can delete group property", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, groupsDeletePropertyUrl), func(req *http.Request) (*http.Response, error) {
// 			require.NoError(t, req.ParseForm())
// 			data := req.Form.Get("data")

// 			var postBody []groupDeletePropertyPayload
// 			require.NoError(t, json.Unmarshal([]byte(data), &postBody))
// 			require.Len(t, postBody, 1)

// 			groupDeleteProperty := postBody[0]
// 			require.Equal(t, "token", groupDeleteProperty.Token)
// 			require.Equal(t, "group-key", groupDeleteProperty.GroupKey)
// 			require.Equal(t, "group-id", groupDeleteProperty.GroupId)

// 			require.Equal(t, []string{"some-prop"}, groupDeleteProperty.Unset)

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.GroupDeleteProperty(ctx, "group-key", "group-id", []string{"some-prop"}))
// 	})
// }

// func TestGroupRemoveListProperty(t *testing.T) {
// 	t.Run("can remove list property", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, groupsRemoveFromListPropertyUrl), func(req *http.Request) (*http.Response, error) {
// 			require.NoError(t, req.ParseForm())
// 			data := req.Form.Get("data")

// 			var postBody []groupRemoveListPropertyPayload
// 			require.NoError(t, json.Unmarshal([]byte(data), &postBody))
// 			require.Len(t, postBody, 1)

// 			removeList := postBody[0]
// 			require.Equal(t, "token", removeList.Token)
// 			require.Equal(t, "group-key", removeList.GroupKey)
// 			require.Equal(t, "group-id", removeList.GroupId)

// 			require.Equal(t, "value", removeList.Remove["list"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.GroupRemoveListProperty(ctx, "group-key", "group-id", map[string]any{
// 			"list": "value",
// 		}))
// 	})
// }

// func TestGroupUnionListProperty(t *testing.T) {
// 	t.Run("can union list", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, groupsUnionListPropertyUrl), func(req *http.Request) (*http.Response, error) {
// 			require.NoError(t, req.ParseForm())
// 			data := req.Form.Get("data")

// 			var postBody []groupUnionListPropertyPayload
// 			require.NoError(t, json.Unmarshal([]byte(data), &postBody))
// 			require.Len(t, postBody, 1)

// 			unionList := postBody[0]
// 			require.Equal(t, "token", unionList.Token)
// 			require.Equal(t, "group-key", unionList.GroupKey)
// 			require.Equal(t, "group-id", unionList.GroupId)

// 			require.Equal(t, "value", unionList.Union["list"])

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.GroupUnionListProperty(ctx, "group-key", "group-id", map[string]any{
// 			"list": "value",
// 		}))
// 	})
// }

// func TestGroupDelete(t *testing.T) {
// 	t.Run("can delete group", func(t *testing.T) {
// 		ctx := context.Background()

// 		httpmock.Activate()
// 		defer httpmock.DeactivateAndReset()

// 		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", usEndpoint, groupsDeleteGroupUrl), func(req *http.Request) (*http.Response, error) {
// 			require.NoError(t, req.ParseForm())
// 			data := req.Form.Get("data")

// 			var postBody []groupDeletePayload
// 			require.NoError(t, json.Unmarshal([]byte(data), &postBody))
// 			require.Len(t, postBody, 1)

// 			delete := postBody[0]
// 			require.Equal(t, "token", delete.Token)
// 			require.Equal(t, "group-key", delete.GroupKey)
// 			require.Equal(t, "group-id", delete.GroupId)

// 			body := `
// 			1
// 			`

// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       io.NopCloser(strings.NewReader(body)),
// 			}, nil
// 		})

// 		mp := NewClient("token")
// 		require.NoError(t, mp.GroupDelete(ctx, "group-key", "group-id"))
// 	})
// }
