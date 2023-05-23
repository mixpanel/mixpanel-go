package mixpanel

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMixpanelOptions(t *testing.T) {
	t.Run("eu residency", func(t *testing.T) {
		mp := NewClient("", EuResidency())
		require.Equal(t, mp.apiEndpoint, euEndpoint)
		require.Equal(t, mp.dataEndpoint, euDataEndpoint)
	})

	t.Run("project id", func(t *testing.T) {
		mp := NewClient("", ProjectID(117))
		require.Equal(t, 117, mp.projectID)
	})

	t.Run("api secret", func(t *testing.T) {
		mp := NewClient("", ApiSecret("api-secret"))
		require.Equal(t, "api-secret", mp.apiSecret)
	})

	t.Run("service account", func(t *testing.T) {
		mp := NewClient("", ServiceAccount("username", "secret"))
		require.NotNil(t, mp.serviceAccount)
		require.Equal(t, "username", mp.serviceAccount.Username)
		require.Equal(t, "secret", mp.serviceAccount.Secret)
	})

	t.Run("set api proxy", func(t *testing.T) {
		mp := NewClient("", ProxyApiLocation("https://localhost:8080"))
		require.Equal(t, "https://localhost:8080", mp.apiEndpoint)
	})

	t.Run("set data proxy", func(t *testing.T) {
		mp := NewClient("", ProxyDataLocation("https://localhost:8080"))
		require.Equal(t, "https://localhost:8080", mp.dataEndpoint)
	})

	t.Run("debug http", func(t *testing.T) {
		mp := NewClient("", DebugHttpCalls(os.Stdout))
		require.NotNil(t, mp.debugHttpCall)
	})

	t.Run("http client", func(t *testing.T) {
		mp := NewClient("", HttpClient(nil))
		require.Nil(t, mp.client)
	})
}
