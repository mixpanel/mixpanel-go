package mixpanel

import (
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
