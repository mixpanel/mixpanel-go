package examples

import (
	"context"
	"fmt"
	"log"

	mixpanel "github.com/mixpanel/mixpanel-go/v2"
	"github.com/mixpanel/mixpanel-go/v2/flags"
)

func InvokeLocalFlagsSample() {
	ctx := context.Background()

	config := flags.DefaultLocalFlagsConfig()
	config.EnablePolling = true

	client := mixpanel.NewApiClient("YOUR_PROJECT_TOKEN", mixpanel.WithLocalFlags(config))

	if err := client.LocalFlags.StartPollingForDefinitions(ctx); err != nil {
		log.Fatalf("Failed to start polling: %v", err)
	}
	defer client.LocalFlags.StopPollingForDefinitions()

	// Define the user context for flag evaluation
	userContext := flags.FlagContext{
		"distinct_id": "user-123",
		"custom_properties": map[string]any{
			"plan":    "premium",
			"country": "US",
		},
	}

	// Get a variant value with a fallback
	variantValue, err := client.LocalFlags.GetVariantValue(ctx, "new-feature", "default-value", userContext)
	if err != nil {
		log.Printf("Error getting variant: %v", err)
	}
	fmt.Printf("Variant value: %v\n", variantValue)
}
