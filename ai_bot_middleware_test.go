package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestBotClassifyingIngestion(t *testing.T) {
	setupTrackEndpoint := func(t *testing.T, client *ApiClient, testPayload func([]*Event)) {
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)
		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, trackURL),
			func(req *http.Request) (*http.Response, error) {
				var r []*Event
				require.NoError(t, json.NewDecoder(req.Body).Decode(&r))
				testPayload(r)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"error":"","status":1}`)),
				}, nil
			},
		)
	}

	t.Run("enriches Track calls when $user_agent identifies an AI bot", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token")
		mp := NewBotClassifyingIngestion(inner)
		event := inner.NewEvent("page_view", "user123", map[string]any{
			"$user_agent": "Mozilla/5.0 (compatible; GPTBot/1.2; +https://openai.com/gptbot)",
		})
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			props := events[0].Properties
			require.Equal(t, true, props["$is_ai_bot"])
			require.Equal(t, "GPTBot", props["$ai_bot_name"])
			require.Equal(t, "OpenAI", props["$ai_bot_provider"])
			require.Equal(t, "indexing", props["$ai_bot_category"])
		})
		require.NoError(t, mp.Track(ctx, []*Event{event}))
	})

	t.Run("sets $is_ai_bot false when user agent is not an AI bot", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token")
		mp := NewBotClassifyingIngestion(inner)
		event := inner.NewEvent("page_view", "user123", map[string]any{
			"$user_agent": "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36",
		})
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			props := events[0].Properties
			require.Equal(t, false, props["$is_ai_bot"])
			_, hasName := props["$ai_bot_name"]
			require.False(t, hasName)
		})
		require.NoError(t, mp.Track(ctx, []*Event{event}))
	})

	t.Run("does not add bot properties when $user_agent is absent", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token")
		mp := NewBotClassifyingIngestion(inner)
		event := inner.NewEvent("page_view", "user123", map[string]any{
			"page_url": "/products",
		})
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			_, hasBot := events[0].Properties["$is_ai_bot"]
			require.False(t, hasBot)
		})
		require.NoError(t, mp.Track(ctx, []*Event{event}))
	})

	t.Run("preserves existing properties alongside bot classification", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token")
		mp := NewBotClassifyingIngestion(inner)
		event := inner.NewEvent("page_view", "user123", map[string]any{
			"$user_agent": "GPTBot/1.2",
			"page_url":    "/products",
			"custom_prop": "value",
		})
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			props := events[0].Properties
			require.Equal(t, "/products", props["page_url"])
			require.Equal(t, "value", props["custom_prop"])
			require.Equal(t, true, props["$is_ai_bot"])
			require.Equal(t, "token", props["token"])
			require.Equal(t, "user123", props["distinct_id"])
			require.Equal(t, "go", props["mp_lib"])
		})
		require.NoError(t, mp.Track(ctx, []*Event{event}))
	})

	t.Run("classifies multiple events in a batch", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token")
		mp := NewBotClassifyingIngestion(inner)
		events := []*Event{
			inner.NewEvent("page_view", "bot1", map[string]any{"$user_agent": "GPTBot/1.2"}),
			inner.NewEvent("page_view", "user1", map[string]any{"$user_agent": "Mozilla/5.0 Chrome/120"}),
			inner.NewEvent("page_view", "user2", map[string]any{"page_url": "/home"}),
		}
		setupTrackEndpoint(t, inner, func(received []*Event) {
			require.Len(t, received, 3)
			require.Equal(t, true, received[0].Properties["$is_ai_bot"])
			require.Equal(t, "GPTBot", received[0].Properties["$ai_bot_name"])
			require.Equal(t, false, received[1].Properties["$is_ai_bot"])
			_, hasBot := received[2].Properties["$is_ai_bot"]
			require.False(t, hasBot)
		})
		require.NoError(t, mp.Track(ctx, events))
	})

	t.Run("delegates Import to inner unchanged", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token", ServiceAccount(117, "user", "secret"))
		mp := NewBotClassifyingIngestion(inner)
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)
		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", inner.apiEndpoint, importURL),
			httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{
				"code": 200, "num_records_imported": 1, "status": 1,
			}),
		)
		event := inner.NewEvent("import_event", "user123", map[string]any{"$user_agent": "GPTBot/1.2"})
		success, err := mp.Import(ctx, []*Event{event}, ImportOptions{Strict: false, Compression: None})
		require.NoError(t, err)
		require.Equal(t, 1, success.NumRecordsImported)
	})

	t.Run("delegates PeopleSet to inner unchanged", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token")
		mp := NewBotClassifyingIngestion(inner)
		setupPeopleAndGroupsEndpoint(t, inner, peopleSetURL, func(body io.Reader) {}, peopleAndGroupSuccess())
		people := NewPeopleProperties("some-id", map[string]any{"key": "value"})
		require.NoError(t, mp.PeopleSet(ctx, []*PeopleProperties{people}))
	})
}

func TestBotClassifyingIngestionWithCustomClassifier(t *testing.T) {
	setupTrackEndpoint := func(t *testing.T, client *ApiClient, testPayload func([]*Event)) {
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)
		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, trackURL),
			func(req *http.Request) (*http.Response, error) {
				var r []*Event
				require.NoError(t, json.NewDecoder(req.Body).Decode(&r))
				testPayload(r)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"error":"","status":1}`)),
				}, nil
			},
		)
	}

	t.Run("uses custom classifier when provided", func(t *testing.T) {
		ctx := context.Background()
		inner := NewApiClient("token")
		classifier := NewClassifier([]BotEntry{{
			Pattern:  regexp.MustCompile(`(?i)MyBot/`),
			Name:     "MyBot",
			Provider: "MyCorp",
			Category: "agent",
		}})
		mp := NewBotClassifyingIngestionWithClassifier(inner, classifier)
		event := inner.NewEvent("page_view", "user123", map[string]any{"$user_agent": "MyBot/1.0"})
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			props := events[0].Properties
			require.Equal(t, true, props["$is_ai_bot"])
			require.Equal(t, "MyBot", props["$ai_bot_name"])
			require.Equal(t, "MyCorp", props["$ai_bot_provider"])
			require.Equal(t, "agent", props["$ai_bot_category"])
		})
		require.NoError(t, mp.Track(ctx, []*Event{event}))
	})
}

func TestBotClassificationMiddleware(t *testing.T) {
	t.Run("stores bot classification in context for AI bot", func(t *testing.T) {
		var captured *BotClassification
		handler := BotClassificationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = BotClassificationFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "GPTBot/1.2")
		handler.ServeHTTP(httptest.NewRecorder(), req)
		require.NotNil(t, captured)
		require.True(t, captured.IsAIBot)
		require.Equal(t, "GPTBot", captured.BotName)
		require.Equal(t, "OpenAI", captured.Provider)
	})

	t.Run("stores non-bot classification in context", func(t *testing.T) {
		var captured *BotClassification
		handler := BotClassificationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = BotClassificationFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120")
		handler.ServeHTTP(httptest.NewRecorder(), req)
		require.NotNil(t, captured)
		require.False(t, captured.IsAIBot)
	})

	t.Run("returns zero-value classification when no User-Agent header", func(t *testing.T) {
		var captured *BotClassification
		handler := BotClassificationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = BotClassificationFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)
		require.NotNil(t, captured)
		require.False(t, captured.IsAIBot)
	})

	t.Run("does not modify the response", func(t *testing.T) {
		handler := BotClassificationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "value")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("Created"))
		}))
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("User-Agent", "GPTBot/1.2")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
		require.Equal(t, "value", w.Header().Get("X-Custom"))
		require.Equal(t, "Created", w.Body.String())
	})

	t.Run("BotClassificationFromContext returns nil without middleware", func(t *testing.T) {
		result := BotClassificationFromContext(context.Background())
		require.Nil(t, result)
	})
}

func TestTrackRequest(t *testing.T) {
	setupTrackEndpoint := func(t *testing.T, client *ApiClient, testPayload func([]*Event)) {
		httpmock.Activate()
		t.Cleanup(httpmock.DeactivateAndReset)
		httpmock.RegisterResponder(http.MethodPost, fmt.Sprintf("%s%s", client.apiEndpoint, trackURL),
			func(req *http.Request) (*http.Response, error) {
				var r []*Event
				require.NoError(t, json.NewDecoder(req.Body).Decode(&r))
				testPayload(r)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"error":"","status":1}`)),
				}, nil
			},
		)
	}

	t.Run("extracts user-agent and IP from http.Request", func(t *testing.T) {
		inner := NewApiClient("token")
		mp := NewBotClassifyingIngestion(inner)
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			props := events[0].Properties
			require.Equal(t, "GPTBot/1.2", props["$user_agent"])
			require.Equal(t, true, props["$is_ai_bot"])
			require.Equal(t, "GPTBot", props["$ai_bot_name"])
			require.Equal(t, "/api/products", props["page_url"])
		})
		httpReq := httptest.NewRequest(http.MethodGet, "/api/products", nil)
		httpReq.Header.Set("User-Agent", "GPTBot/1.2")
		httpReq.RemoteAddr = "1.2.3.4:12345"
		event := inner.NewEvent("page_view", "user123", map[string]any{"page_url": "/api/products"})
		TrackRequest(httpReq, event)
		require.NoError(t, mp.Track(httpReq.Context(), []*Event{event}))
	})

	t.Run("extracts IP from X-Forwarded-For when present", func(t *testing.T) {
		inner := NewApiClient("token")
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			require.Equal(t, "5.6.7.8", events[0].Properties["ip"].(string)) // "ip" matches the propertyIP constant defined in mixpanel.go:28
		})
		httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
		httpReq.Header.Set("User-Agent", "Chrome/120")
		httpReq.Header.Set("X-Forwarded-For", "5.6.7.8, 9.10.11.12")
		httpReq.RemoteAddr = "127.0.0.1:80"
		event := inner.NewEvent("page_view", "user123", nil)
		TrackRequest(httpReq, event)
		require.NoError(t, inner.Track(httpReq.Context(), []*Event{event}))
	})

	t.Run("handles request with no user-agent header", func(t *testing.T) {
		inner := NewApiClient("token")
		setupTrackEndpoint(t, inner, func(events []*Event) {
			require.Len(t, events, 1)
			_, hasUA := events[0].Properties["$user_agent"]
			require.False(t, hasUA)
		})
		httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
		httpReq.Header.Del("User-Agent")
		event := inner.NewEvent("page_view", "user123", nil)
		TrackRequest(httpReq, event)
		require.NoError(t, inner.Track(httpReq.Context(), []*Event{event}))
	})
}
