package mixpanel

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"cloud.google.com/go/civil"
)

const (
	exportUrl = "/api/2.0/export"
)

func (m *Mixpanel) Export(ctx context.Context, fromDate, toDate civil.Date, limit int, event, where string) ([]*Event, error) {
	query := url.Values{}
	query.Add("project_id", strconv.Itoa(m.projectID))
	query.Add("from_date", fromDate.String())
	query.Add("to_date", fromDate.String())
	if limit != 0 {
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
		var e []*Event

		scanner := bufio.NewScanner(httpResponse.Body)
		for scanner.Scan() {
			var event *Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				return nil, fmt.Errorf("failed to parse event: %v", err)
			}
			e = append(e, event)
		}
		return e, nil

	default:
		return nil, fmt.Errorf("unexpected status code: %d", httpResponse.StatusCode)
	}
}
