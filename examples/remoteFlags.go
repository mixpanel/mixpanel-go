package examples

import (
	"context"
	"fmt"
	"log"

	mixpanel "github.com/mixpanel/mixpanel-go/v2"
	"github.com/mixpanel/mixpanel-go/v2/flags"
)

func InvokeRemoteFlagsSample() {
	ctx := context.Background()

	config := flags.DefaultRemoteFlagsConfig()

	client := mixpanel.NewApiClient("YOUR_PROJECT_TOKEN", mixpanel.WithRemoteFlags(config))

	// Define the user context for flag evaluation
	userContext := flags.FlagContext{
		"distinct_id": "user-123",
		"custom_properties": map[string]any{
			"plan":    "premium",
			"country": "US",
		},
	}

	// Get a variant value with a fallback
	// Note that remote evaluation makes a server request for each call
	variantValue, err := client.RemoteFlags.GetVariantValue(ctx, "new-feature", "default-value", userContext)
	if err != nil {
		log.Printf("Error getting variant: %v", err)
	}
	fmt.Printf("Variant value: %v\n", variantValue)
}
