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
	"net/http/httputil"
	"net/url"
	"strconv"
)

type MpCompression int

var (
	None MpCompression = 0
	Gzip MpCompression = 1
)

var (
	ErrUnexpectedStatus = errors.New("unexpected status code")

	apiErrorStatus = 0
)

type HttpError struct {
	Status int
	Body   string
}

func newHttpError(statusCode int, data io.Reader) error {
	body, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	return HttpError{
		Status: statusCode,
		Body:   string(body),
	}
}

func (h HttpError) Error() string {
	return fmt.Sprintf("unexpected status code: %d, body: %s", h.Status, h.Body)
}

func (h HttpError) Unwrap() error {
	return ErrUnexpectedStatus
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

func (m *ApiClient) importAuthOptions() httpOptions {
	return func(req *http.Request) {
		if m.serviceAccount != nil {
			req.SetBasicAuth(m.serviceAccount.Username, m.serviceAccount.Secret)
		} else if m.apiSecret != "" {
			req.SetBasicAuth(m.apiSecret, "")
		} else {
			req.SetBasicAuth(m.token, "")
		}
	}
}

func (m *ApiClient) useApiSecret() httpOptions {
	return func(req *http.Request) {
		req.SetBasicAuth(m.apiSecret, "")
	}
}

// exportServiceAccount uses the service account if available and adds the query params
// or falls back to apiSecret
func (m *ApiClient) exportServiceAccount() httpOptions {
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

func (d *debugHttpCalls) writeDebug(r *http.Request) error {
	if d.writer == nil {
		return nil
	}

	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		return fmt.Errorf("failed to dump request %w", err)
	}

	_, err = d.writer.Write([]byte("-----Start Request-----\n"))
	if err != nil {
		return fmt.Errorf("failed to write start header %w", err)
	}

	_, err = d.writer.Write(requestDump)
	if err != nil {
		return fmt.Errorf("failed to write debug_http payload %w", err)
	}

	_, err = d.writer.Write([]byte("\n-----End Request-----\n\n"))
	if err != nil {
		return fmt.Errorf("failed to write end header %w", err)
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

type requestPostPayloadType int

const (
	jsonPayload requestPostPayloadType = iota
	formPayload
)

func makeRequestBody(body any, bodyType requestPostPayloadType, compress MpCompression) (*bytes.Reader, error) {
	if body == nil {
		return nil, fmt.Errorf("body is nil")
	}

	var err error
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to create http body: %w", err)
	}

	switch bodyType {
	case jsonPayload:
		return requestBodyJsonCompress(jsonData, compress)
	case formPayload:
		return requestForm(jsonData)
	default:
		return nil, fmt.Errorf("unknown body type: %d", bodyType)
	}
}

func requestBodyJsonCompress(jsonPayload []byte, compress MpCompression) (*bytes.Reader, error) {
	switch compress {
	case None:
		return bytes.NewReader(jsonPayload), nil
	case Gzip:
		jsonData, err := gzipBody(jsonPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to gzip body: %w", err)
		}
		return bytes.NewReader(jsonData), nil
	default:
		return nil, fmt.Errorf("unknown compression type: %d", compress)
	}
}

func requestForm(jsonPayload []byte) (*bytes.Reader, error) {
	form := url.Values{}
	form.Add("data", string(jsonPayload))
	return bytes.NewReader([]byte(form.Encode())), nil
}

func (m *ApiClient) doRequestBody(
	ctx context.Context,
	method string,
	requestUrl string,
	body io.Reader,
	options ...httpOptions,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestUrl, body)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	for _, o := range options {
		o(request)
	}

	if err := m.debugHttpCall.writeDebug(request); err != nil {
		return nil, fmt.Errorf("failed to write debug_http call: %w", err)
	}

	return m.client.Do(request)
}

func (m *ApiClient) doPeopleRequest(ctx context.Context, body any, u string) error {
	requestBody, err := makeRequestBody(body, jsonPayload, None)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}
	response, err := m.doRequestBody(
		ctx,
		http.MethodPost,
		m.apiEndpoint+u,
		requestBody,
		acceptPlainText(),
		applicationJsonHeader(),
	)

	if err != nil {
		return fmt.Errorf("failed to post request: %w", err)
	}
	defer response.Body.Close()

	return processPeopleRequestResponse(response)
}

func (m *ApiClient) doIdentifyRequest(ctx context.Context, body any, u string, option ...httpOptions) error {
	requestBody, err := makeRequestBody(body, formPayload, None)
	if err != nil {
		return fmt.Errorf("failed to create request body: %w", err)
	}

	requestOptions := append([]httpOptions{acceptPlainText(), applicationFormData()}, option...)
	response, err := m.doRequestBody(
		ctx,
		http.MethodPost,
		m.apiEndpoint+u,
		requestBody,
		requestOptions...,
	)

	if err != nil {
		return fmt.Errorf("failed to post request: %w", err)
	}
	defer response.Body.Close()

	return processPeopleRequestResponse(response)
}

func processPeopleRequestResponse(response *http.Response) error {
	switch response.StatusCode {
	case http.StatusOK:
		var code int
		if err := json.NewDecoder(response.Body).Decode(&code); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		if code == apiErrorStatus {
			return errors.New("api return code 0")
		}
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: %w", newHttpError(response.StatusCode, response.Body))
	case http.StatusForbidden:
		return fmt.Errorf("forbidden: %w", newHttpError(response.StatusCode, response.Body))
	default:
		return newHttpError(response.StatusCode, response.Body)
	}
}

type VerboseError struct {
	ApiError string `json:"error"`
	Status   int    `json:"status"`
}

func (a VerboseError) Error() string {
	return a.ApiError
}

func parseVerboseApiError(jsonReader io.Reader) error {
	var r VerboseError
	if err := json.NewDecoder(jsonReader).Decode(&r); err != nil {
		return fmt.Errorf("failed to json decode response body: %w", err)
	}

	if r.Status == apiErrorStatus {
		return r
	}

	return nil
}
