package mixpanel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMixpanelOptions(t *testing.T) {
	t.Run("eu residency", func(t *testing.T) {
		mp := NewClient(0, "", "", EuResidency())
		require.Equal(t, mp.ingestionEndpoint, euEndpoint)
		require.Equal(t, mp.dataEndpoint, euDataEndpoint)
	})
	t.Run("service account", func(t *testing.T) {
		mp := NewClient(0, "", "", SetServiceAccount("username", "secret"))
		require.NotNil(t, mp.serviceAccount)
		require.Equal(t, "username", mp.serviceAccount.Username)
		require.Equal(t, "secret", mp.serviceAccount.Secret)
	})

	t.Run("set proxy", func(t *testing.T) {
		mp := NewClient(0, "", "", ProxyApiLocation("https://localhost:8080"))
		require.Equal(t, "https://localhost:8080", mp.ingestionEndpoint)
	})
	t.Run("debug http", func(t *testing.T) {
		mp := NewClient(0, "", "", DebugHttpCalls())
		require.True(t, mp.debugHttp)
	})
}
