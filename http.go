package mixpanel

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func (m *Mixpanel) executeBasicRequest(ctx context.Context, dataBody any, url string, useServiceAccount bool) error {
	body, err := json.Marshal(dataBody)
	if err != nil {
		return fmt.Errorf("failed to create http body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add(acceptHeader, acceptPlainTextHeader)
	req.Header.Add(contentType, contentTypeJson)

	if m.serviceAccount != nil {
		req.SetBasicAuth(m.serviceAccount.Username, m.serviceAccount.Secret)
	} else {
		req.SetBasicAuth(m.apiSecret, "")
	}

	httpResponse, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post /track request: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("non 200 status code")
	}

	var r GenericError
	if err := json.NewDecoder(httpResponse.Body).Decode(&r); err != nil {
		return fmt.Errorf("failed to json decode response body: %w", err)
	}

	if r.Status == apiErrorStatus {
		return r
	}
	return nil
}

func gzipBody(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzip := gzip.NewWriter(&buf)
	if _, err := gzip.Write(data); err != nil {
		return nil, fmt.Errorf("failed to compress body using gzip: %w", err)
	}
	if err := gzip.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}
