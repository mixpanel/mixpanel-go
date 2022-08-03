package mixpanel

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

var (
	ErrStatusCode = errors.New("unexpected status code")

	apiErrorStatus = 0
)

type VerboseError struct {
	ApiError string `json:"error"`
	Status   int    `json:"status"`
}

func (a VerboseError) Error() string {
	return a.ApiError
}

type PeopleError struct {
	Code int `json:"code"`
}

func (p PeopleError) Error() string {
	return "people update return code 0"
}

func (m *Mixpanel) doRequest(
	ctx context.Context,
	method string,
	dataBody any,
	url string,
	acceptJson, useServiceAccount bool,
	compression MpCompression,
	params url.Values,
	headers http.Header,
) (*http.Response, error) {
	var payloadBody []byte
	var uncompressedBody []byte
	var contentHeader string

	if dataBody != nil {
		var err error
		uncompressedBody, err = json.Marshal(dataBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create http body: %w", err)
		}
		payloadBody = uncompressedBody

		switch compression {
		case Gzip:
			payloadBody, err = gzipBody(uncompressedBody)
			if err != nil {
				return nil, fmt.Errorf("failed to compress: %w", err)
			}
			contentHeader = "gzip"
		}
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		url,
		bytes.NewReader(payloadBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if contentHeader != "" {
		req.Header.Add(contentEncodingHeader, contentHeader)
	}

	if acceptJson {
		req.Header.Set(acceptHeader, acceptJsonHeader)
	} else {
		req.Header.Set(acceptHeader, acceptPlainTextHeader)
	}

	if m.serviceAccount != nil {
		req.SetBasicAuth(m.serviceAccount.Username, m.serviceAccount.Secret)
	} else {
		req.SetBasicAuth(m.apiSecret, "")
	}

	query := req.URL.Query()
	for k, v := range params {
		query[k] = v
	}
	req.URL.RawQuery = query.Encode()

	for k, v := range headers {
		req.Header[k] = v
	}

	if m.debugHttp {
		fmt.Printf("Url -> %s\nPayload -> %s \n", req.URL.String(), uncompressedBody)
		fmt.Println("Headers")
		for h, v := range req.Header {
			fmt.Println(h, v)
		}
	}

	httpResponse, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to post request: %w", err)
	}

	return httpResponse, nil
}

func (m *Mixpanel) doPeopleRequest(ctx context.Context, body any, u string) error {
	response, err := m.doRequest(
		ctx,
		http.MethodPost,
		body,
		m.baseEndpoint+u,
		false,
		false,
		None,
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to post request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		var code int
		if err := json.NewDecoder(response.Body).Decode(&code); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		if code == 0 {
			return errors.New("code 0")
		}
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return returnVerboseError(response)
	default:
		return ErrStatusCode
	}
}

func returnVerboseError(httpResponse *http.Response) error {
	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("non 200 status code")
	}

	var r VerboseError
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
