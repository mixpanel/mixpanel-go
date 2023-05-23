package mixpanel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMixpanelOptions(t *testing.T) {
	t.Run("eu residency", func(t *testing.T) {
		mp := NewClient("", EuResidency())
		require.Equal(t, mp.apiEndpoint, euEndpoint)
		require.Equal(t, mp.dataEndpoint, euDataEndpoint)
	})
	t.Run("service account", func(t *testing.T) {
		mp := NewClient("", SetServiceAccount("username", "secret"))
		require.NotNil(t, mp.serviceAccount)
		require.Equal(t, "username", mp.serviceAccount.Username)
		require.Equal(t, "secret", mp.serviceAccount.Secret)
	})

	t.Run("set proxy", func(t *testing.T) {
		mp := NewClient("", ProxyApiLocation("https://localhost:8080"))
		require.Equal(t, "https://localhost:8080", mp.apiEndpoint)
	})
	t.Run("debug http", func(t *testing.T) {
		mp := NewClient("", DebugHttpCalls())
		require.True(t, mp.debugHttp)
	})
}

func TestMixpanelNewEvent(t *testing.T) {
	mp := NewClient("")
	event := mp.NewEvent("some event", EmptyDistinctID, nil)
	require.NotNil(t, event.Properties)
}
