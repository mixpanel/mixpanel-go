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

func (m *Mixpanel) useServiceAccountOrProjectSecret() httpOptions {
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

type debugHttpCall struct {
	Url     string
	Query   url.Values
	Headers http.Header
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

func (m *Mixpanel) doRequestBody(
	ctx context.Context,
	method string,
	requestUrl string,
	body io.Reader,
	options ...httpOptions,
) (*http.Response, error) {
	var debugHttpCall debugHttpCall

	request, err := http.NewRequestWithContext(ctx, method, requestUrl, body)
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

func (m *Mixpanel) doPeopleRequest(ctx context.Context, body any, u string) error {
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

func (m *Mixpanel) doIdentifyRequest(ctx context.Context, body any, u string, option ...httpOptions) error {
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
		if code == 0 {
			return errors.New("code 0")
		}
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: %w", parseVerboseApiError(response.Body))
	case http.StatusForbidden:
		return fmt.Errorf("forbidden: %w", parseVerboseApiError(response.Body))
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
