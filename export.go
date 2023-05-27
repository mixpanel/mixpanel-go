package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	exportUrl = "/api/2.0/export"

	ExportNoLimit       int    = 0
	ExportNoEventFilter string = ""
	ExportNoWhereFilter string = ""
)

// Export calls the Raw Export API
// https://developer.mixpanel.com/reference/raw-event-export
func (a *ApiClient) Export(ctx context.Context, fromDate, toDate time.Time, limit int, event, where string) ([]*Event, error) {
	query := url.Values{}
	query.Add("from_date", fromDate.Format("2006-01-02"))
	query.Add("to_date", toDate.Format("2006-01-02"))
	if limit != ExportNoLimit {
		query.Add("limit", strconv.Itoa(limit))
	}
	if event != "" {
		query.Add("event", event)
	}
	if where != "" {
		query.Add("where", where)
	}

	httpResponse, err := a.doRequestBody(
		ctx,
		http.MethodGet,
		a.dataEndpoint+exportUrl,
		nil,
		a.exportServiceAccount(), acceptPlainText(), addQueryParams(query),
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
		return nil, newHttpError(httpResponse.StatusCode, httpResponse.Body)
	}
}
