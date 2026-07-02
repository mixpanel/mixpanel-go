# Mixpanel OpenFeature Provider for Go

##### _May 13, 2026_ - [openfeature/v0.1.0](https://github.com/mixpanel/mixpanel-go/releases/tag/openfeature/v0.1.0)

An [OpenFeature](https://openfeature.dev/) provider that wraps Mixpanel's feature flags for use with the OpenFeature Go SDK. This allows you to use Mixpanel's feature flagging capabilities through OpenFeature's standardized, vendor-agnostic API.

## Overview

This package provides a bridge between Mixpanel's native feature flags implementation and the OpenFeature specification. By using this provider, you can:

- Leverage Mixpanel's powerful feature flag and experimentation platform
- Use OpenFeature's standardized API for flag evaluation
- Easily switch between feature flag providers without changing your application code
- Integrate with OpenFeature's ecosystem of tools and frameworks

## Installation

```bash
go get github.com/mixpanel/mixpanel-go/openfeature
```

You will also need the OpenFeature Go SDK:

```bash
go get github.com/open-feature/go-sdk
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	mixpanelopenfeature "github.com/mixpanel/mixpanel-go/openfeature"
	"github.com/mixpanel/mixpanel-go/v2/flags"
	of "github.com/open-feature/go-sdk/openfeature"
)

func main() {
	// 1. Create the Mixpanel OpenFeature provider with local evaluation
	provider, err := mixpanelopenfeature.NewProviderWithLocalConfig("YOUR_PROJECT_TOKEN", flags.LocalFlagsConfig{})
	if err != nil {
		panic(err)
	}

	// 2. Register the provider with OpenFeature
	of.SetProvider(provider)
	client := of.NewClient("my-app")

	// 3. Evaluate flags
	showNewFeature, _ := client.BooleanValue(context.Background(), "new-feature-flag", false, of.EvaluationContext{})

	if showNewFeature {
		fmt.Println("New feature is enabled!")
	}
}
```

## Provider Configuration

There are three ways to create a provider, depending on how you want flag evaluation to work:

### Local Evaluation (Recommended)

Flag definitions are fetched from Mixpanel and evaluated locally. This is faster and works offline after the initial fetch.

```go
provider, err := mixpanelopenfeature.NewProviderWithLocalConfig("YOUR_PROJECT_TOKEN", flags.LocalFlagsConfig{})
```

### Remote Evaluation

Each flag evaluation makes a request to Mixpanel's servers. This ensures you always have the latest flag values but requires network connectivity.

```go
provider, err := mixpanelopenfeature.NewProviderWithRemoteConfig("YOUR_PROJECT_TOKEN", flags.RemoteFlagsConfig{})
```

### From an Existing Mixpanel SDK Instance

If you already have a Mixpanel `ApiClient` with a flags provider configured, you can wrap it directly rather than creating a new one:

```go
mp := mixpanel.NewApiClient("YOUR_TOKEN", mixpanel.WithLocalFlags(flags.LocalFlagsConfig{}))
mp.LocalFlags.StartPollingForDefinitions(context.Background())

// Wrap the existing flags provider for use with OpenFeature
provider, err := mixpanelopenfeature.NewProvider(mp.LocalFlags)
```

This also works with `mp.RemoteFlags` for remote evaluation.

## Usage Examples

### Boolean Flag (Feature Gate)

```go
enabled, _ := client.BooleanValue(ctx, "new-checkout", false, of.EvaluationContext{})

if enabled {
	// Show the new checkout experience
}
```

### String Flag (Experiment)

```go
buttonColor, _ := client.StringValue(ctx, "button-color-test", "blue", of.EvaluationContext{})
```

### Numeric Flags

```go
// Float
threshold, _ := client.FloatValue(ctx, "score-threshold", 0.5, of.EvaluationContext{})

// Integer
maxItems, _ := client.IntValue(ctx, "max-items", 10, of.EvaluationContext{})
```

### Object Flag (Dynamic Config)

```go
config, _ := client.ObjectValue(ctx, "homepage-layout", map[string]any{
	"layout":      "grid",
	"itemsPerRow": 3,
}, of.EvaluationContext{})
```

### Mixpanel Flag Types and OpenFeature Evaluation Methods

Mixpanel feature flags support three flag types. Use the corresponding OpenFeature evaluation method based on your flag's variant values:

| Mixpanel Flag Type | Variant Values | OpenFeature Method |
|---|---|---|
| Feature Gate | `true` / `false` | `BooleanValue()` |
| Experiment | boolean, string, number, or JSON object | `BooleanValue()`, `StringValue()`, `FloatValue()`, `IntValue()`, or `ObjectValue()` |
| Dynamic Config | JSON object | `ObjectValue()` |

### Getting Full Resolution Details

If you need additional metadata about the flag evaluation:

```go
details, _ := client.BooleanValueDetails(ctx, "my-feature", false, of.EvaluationContext{})

fmt.Println(details.Value)          // The resolved value
fmt.Println(details.Variant)        // The variant key from Mixpanel
fmt.Println(details.Reason)         // Why this value was returned
fmt.Println(details.ErrorCode)      // Error code if evaluation failed
```

### Passing Evaluation Context

You can pass evaluation context that will be sent to Mixpanel for flag evaluation:

```go
evalCtx := of.NewEvaluationContext("user-123", map[string]any{
	"email": "user@example.com",
	"plan":  "premium",
})

enabled, _ := client.BooleanValue(ctx, "premium-feature", false, evalCtx)
```

### Accessing the Underlying Mixpanel Client

When using `NewProviderWithLocalConfig` or `NewProviderWithRemoteConfig`, the underlying Mixpanel `ApiClient` is available via the `Mixpanel` field. This is useful for tracking events or managing user identity alongside flag evaluation:

```go
provider, _ := mixpanelopenfeature.NewProviderWithLocalConfig("YOUR_TOKEN", flags.LocalFlagsConfig{})

// Use the Mixpanel client directly for tracking, identity, etc.
provider.Mixpanel.Track(ctx, []*mixpanel.Event{
	{Name: "button_clicked", Properties: map[string]any{"color": "blue"}},
})
```

### Shutdown

When your application is shutting down, call `Shutdown()` to stop background polling (for local evaluation):

```go
provider.Shutdown()
```

### Async Exposure Tracking

By default, every flag evaluation tracks an exposure event inline — the `/track` HTTP round trip happens on the calling goroutine before the evaluation method returns. For latency-sensitive code paths, set `ExposureExecutor` on the config so exposure tracking runs off-goroutine:

```go
config := flags.DefaultLocalFlagsConfig()
config.ExposureExecutor = func(send func()) { go send() }

provider, _ := mixpanelopenfeature.NewProviderWithLocalConfig("YOUR_TOKEN", config)
```

For bounded concurrency that never blocks the caller, use a non-blocking
`select` — exposures are dropped once the in-flight cap is reached:

```go
sem := make(chan struct{}, 4)
config.ExposureExecutor = func(send func()) {
    select {
    case sem <- struct{}{}:
        go func() {
            defer func() { <-sem }()
            send()
        }()
    default:
        // At capacity — drop the exposure rather than stall the caller.
    }
}
```

If you'd rather queue than drop, back the executor with a pre-spawned
worker pool that reads from a buffered channel — that keeps the caller
non-blocking as long as the queue has room, and lets you decide the
buffer size and worker count explicitly.

Available on both `LocalFlagsConfig` and `RemoteFlagsConfig`. Defaults to `nil` (inline behavior); existing setups are unaffected.

## Context Mapping

All properties in the OpenFeature `EvaluationContext` are passed directly to Mixpanel's flag evaluation. There is no transformation or filtering of properties.

**`targetingKey` is not special.** Unlike some feature flag providers, `targetingKey` is not used as a special bucketing key in Mixpanel. It is passed as another context property. Mixpanel's server-side configuration determines which properties are used for targeting rules and bucketing.

## Error Handling

The provider uses OpenFeature's standard error codes:

| Error Code | When |
|---|---|
| `PROVIDER_NOT_READY` | Flags evaluated before the local provider has finished fetching definitions |
| `FLAG_NOT_FOUND` | The requested flag does not exist in Mixpanel |
| `TYPE_MISMATCH` | The flag value type does not match the requested type (e.g., calling `BooleanValue` on a string flag) |

On any error, the default value you provided is returned.

## FAQ

### Why do flags always return the default value?

1. **Provider not ready (local evaluation):** The provider needs to fetch flag definitions before it can evaluate. Make sure polling has started and definitions have been received.
2. **Flag not configured:** Verify the flag exists in your Mixpanel project and is enabled.
3. **Network issues:** For remote evaluation, check that your application can reach Mixpanel's API.

### Why am I getting TYPE_MISMATCH errors?

The flag's value type in Mixpanel must match the evaluation method you call. For example, if a flag's value is the string `"true"`, use `StringValue()` instead of `BooleanValue()`. For JSON objects, use `ObjectValue()`.

### How does numeric type coercion work?

The provider handles numeric type coercion automatically:
- `IntValue()` accepts `int`, `int32`, `int64`, and whole-number `float64`/`float32` values.
- `FloatValue()` accepts `float32`, `float64`, `int`, `int32`, and `int64` values.

If a `float64` has no fractional part (e.g., `42.0`), it can be evaluated as either an int or a float.

### Are exposure events tracked automatically?

Yes. Each flag evaluation calls the underlying Mixpanel SDK with `reportExposure: true`, which tracks `$experiment_started` events in Mixpanel.

### What's the difference between local and remote evaluation?

- **Local evaluation** fetches flag definitions once and evaluates them in-process. It's faster, works offline after the initial fetch, and reduces network calls. Flag definitions are kept up-to-date via background polling.
- **Remote evaluation** sends each evaluation request to Mixpanel's servers. This always uses the latest flag configuration but requires network connectivity and adds latency per evaluation.

## License

Apache-2.0
