package mixpanel

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerboseError(t *testing.T) {
	t.Run("no error occur", func(t *testing.T) {
		verboseApiErrorJson := `
	{
		"error": "",
		"status": 1
	}
	`

		err := parseVerboseApiError(strings.NewReader(verboseApiErrorJson))
		require.NoError(t, err)
	})

	t.Run("an error occur", func(t *testing.T) {
		verboseApiErrorJson := `
	{
		"error": "data, missing or empty",
		"status": 0
	}
	`

		err := parseVerboseApiError(strings.NewReader(verboseApiErrorJson))
		verboseError := &VerboseError{}
		require.ErrorAs(t, err, verboseError)
		require.Equal(t, "data, missing or empty", verboseError.Error())
	})
}

func TestHttpError(t *testing.T) {
	httpBody := strings.NewReader("http body")
	err := newHttpError(http.StatusTeapot, httpBody)

	genericHttpError := &HttpError{}
	require.ErrorAs(t, err, genericHttpError)
	require.Equal(t, http.StatusTeapot, genericHttpError.Status)
	require.Equal(t, "http body", genericHttpError.Body)

	require.ErrorIs(t, err, ErrUnexpectedStatus)
}

func TestProcessPeopleRequestResponse(t *testing.T) {
	t.Run("StatusUnauthorized", func(t *testing.T) {
		response := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body: io.NopCloser(strings.NewReader(`
			{
				"error": "string",
				"status": "error"
			}
			`)),
		}

		err := processPeopleRequestResponse(response)
		httpErr := &HttpError{}
		require.ErrorAs(t, err, httpErr)
		require.Equal(t, http.StatusUnauthorized, httpErr.Status)
	})

	t.Run("StatusForbidden", func(t *testing.T) {
		response := &http.Response{
			StatusCode: http.StatusForbidden,
			Body: io.NopCloser(strings.NewReader(`
			{
				"error": "string",
				"status": "error"
			}
			`)),
		}

		err := processPeopleRequestResponse(response)
		httpErr := &HttpError{}
		require.ErrorAs(t, err, httpErr)
		require.Equal(t, http.StatusForbidden, httpErr.Status)
	})

	t.Run("unknown code", func(t *testing.T) {
		response := &http.Response{
			StatusCode: http.StatusTeapot,
			Body: io.NopCloser(strings.NewReader(`
			{
				"error": "string",
				"status": "error"
			}
			`)),
		}

		err := processPeopleRequestResponse(response)
		httpErr := &HttpError{}
		require.ErrorAs(t, err, httpErr)
		require.Equal(t, http.StatusTeapot, httpErr.Status)
	})
}
