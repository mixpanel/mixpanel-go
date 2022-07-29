package mixpanel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMixpanelEuResidency(t *testing.T) {
	mp := NewClient("", EuResidency())
	require.Equal(t, mp.baseEndpoint, euEndpoint)
}
