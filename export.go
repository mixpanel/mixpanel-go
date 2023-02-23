package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"cloud.google.com/go/civil"
)

const (
	exportUrl = "/api/2.0/export"

	ExportNoLimit       int    = 0
	ExportNoEventFilter string = ""
	ExportNoWhereFilter string = ""
)

// Export calls the Raw Export API
// https://developer.mixpanel.com/reference/raw-event-export

// Example on how to explore everything fromDate - toDate
// Export(ctx, fromDate, toDate, ExportNoLimit, ExportNoEventFilter, ExportNoWhereFilter)
func (m *Mixpanel) Export(ctx context.Context, fromDate, toDate civil.Date, limit int, event, where string) ([]*Event, error) {
	query := url.Values{}
	query.Add("from_date", fromDate.String())
	query.Add("to_date", toDate.String())
	if limit != ExportNoLimit {
		query.Add("limit", strconv.Itoa(limit))
	}
	if event != "" {
		query.Add("event", event)
	}
	if where != "" {
		query.Add("where", where)
	}

	httpResponse, err := m.doRequest(
		ctx,
		http.MethodGet,
		m.dataEndpoint+exportUrl,
		nil,
		None,
		m.exportServiceAccount(), acceptPlainText(), addQueryParams(query),
	)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()

	switch httpResponse.StatusCode {
	case http.StatusOK:
		var results []*Event

		dec := json.NewDecoder(httpResponse.Body)
		for dec.More() {
			var e *Event
			err := dec.Decode(&e)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event:%w", err)
			}
			results = append(results, e)
		}
		return results, nil

	default:
		body, err := io.ReadAll(httpResponse.Body)
		if err != nil {
			return nil, err
		}
		return nil, HttpError{
			Status: httpResponse.StatusCode,
			Body:   string(body),
		}
	}
}
