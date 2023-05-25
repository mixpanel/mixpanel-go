package mixpanel

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

var (
	ErrStatusCode = errors.New("unexpected status code")

	apiErrorStatus = 0
)

type HttpError struct {
	Status int
	Body   string
}

func (h HttpError) Error() string {
	return ErrStatusCode.Error()
}

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

type httpOptions func(req *http.Request)

func gzipHeader() httpOptions {
	return func(req *http.Request) {
		req.Header.Set(contentEncodingHeader, "gzip")
	}
}

func applicationJsonHeader() httpOptions {
	return func(req *http.Request) {
		req.Header.Set(contentTypeHeader, contentTypeApplicationJson)
	}
}

func applicationFormData() httpOptions {
	return func(req *http.Request) {
		req.Header.Set(contentTypeHeader, contentTypeApplicationForm)
	}
}

func (m *Mixpanel) useServiceAccount() httpOptions {
	return func(req *http.Request) {
		if m.serviceAccount != nil {
			req.SetBasicAuth(m.serviceAccount.Username, m.serviceAccount.Secret)
		} else {
			req.SetBasicAuth(m.apiSecret, "")
		}
	}
}

func (m *Mixpanel) useApiSecret() httpOptions {
	return func(req *http.Request) {
		req.SetBasicAuth(m.apiSecret, "")
	}
}

// exportServiceAccount uses the service account if available and adds the query params
// or falls back to apiSecret
func (m *Mixpanel) exportServiceAccount() httpOptions {
	return func(req *http.Request) {
		if m.serviceAccount != nil {
			req.SetBasicAuth(m.serviceAccount.Username, m.serviceAccount.Secret)
			values := url.Values{}
			values.Add("project_id", strconv.Itoa(m.projectID))
			addQueryParams(values)(req)
		} else {
			req.SetBasicAuth(m.apiSecret, "")
		}
	}
}

func acceptJson() httpOptions {
	return func(req *http.Request) {
		req.Header.Set(acceptHeader, acceptJsonHeader)
	}
}

func addQueryParams(query url.Values) httpOptions {
	return func(req *http.Request) {
		rQuery := req.URL.Query()
		for key, values := range query {
			rQuery[key] = values
		}
		req.URL.RawQuery = rQuery.Encode()
	}
}

func acceptPlainText() httpOptions {
	return func(req *http.Request) {
		req.Header.Set(acceptHeader, acceptPlainTextHeader)
	}
}

type debugHttpCalls struct {
	writer io.Writer
}

func (d *debugHttpCalls) writeDebug(call debugHttpCall) error {
	if d.writer == nil {
		return nil
	}

	debugPayload, err := json.MarshalIndent(call, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal debug_http payload %w", err)
	}

	_, err = d.writer.Write(debugPayload)
	if err != nil {
		return fmt.Errorf("failed to write debug_http payload %w", err)
	}
	return nil
}

type debugHttpCall struct {
	RawPayload string
	Url        string
	Query      url.Values
	Headers    http.Header
}

func (m *Mixpanel) doRequestBody(
	ctx context.Context,
	method string,
	reqUrl string,
	body any,
	compress MpCompression,
	options ...httpOptions,
) (*http.Response, error) {
	var debugHttpCall debugHttpCall

	var requestBody []byte
	if body != nil {
		jsonMarshal, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to create http body: %w", err)
		}
		debugHttpCall.RawPayload = string(jsonMarshal)
		requestBody = jsonMarshal

		switch compress {
		case Gzip:
			requestBody, err = gzipBody(jsonMarshal)
			if err != nil {
				return nil, fmt.Errorf("failed to gzip body: %w", err)
			}
			options = append(options, gzipHeader())
		case FormData:
			form := url.Values{}
			form.Add("data", string(jsonMarshal))
			requestBody = []byte(form.Encode())
		}
	}

	request, err := http.NewRequestWithContext(ctx, method, reqUrl, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	for _, o := range options {
		o(request)
	}

	debugHttpCall.Url = request.URL.String()
	debugHttpCall.Query = request.URL.Query()
	debugHttpCall.Headers = request.Header
	if err := m.debugHttpCall.writeDebug(debugHttpCall); err != nil {
		return nil, fmt.Errorf("failed to write debug_http call: %w", err)
	}

	return m.client.Do(request)
}

func (m *Mixpanel) doPeopleRequest(ctx context.Context, body any, u string, compress MpCompression, options ...httpOptions) error {
	response, err := m.doRequestBody(
		ctx,
		http.MethodPost,
		m.apiEndpoint+u,
		body,
		compress,
		options...,
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
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: %w", returnVerboseError(response))
	case http.StatusForbidden:
		return fmt.Errorf("forbidden: %w", returnVerboseError(response))
	default:
		return ErrStatusCode
	}
}

func returnVerboseError(httpResponse *http.Response) error {
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
