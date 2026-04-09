# Releasing the OpenFeature Provider

The OpenFeature provider (`github.com/mixpanel/mixpanel-go/v2/openfeature`) is published as a separate Go module within this repository. It has its own `go.mod` and is versioned independently from the core SDK.

## Releasing

1. Ensure the OpenFeature provider code is on `main`

2. Tag the release using the submodule prefix:
   ```bash
   git tag openfeature/v0.1.0
   git push origin openfeature/v0.1.0
   ```

3. The Go module proxy auto-indexes the new version within minutes

4. Verify at https://pkg.go.dev/github.com/mixpanel/mixpanel-go/v2/openfeature

No build step, credentials, or upload is needed — the Go proxy pulls directly from the git tag.

## Versioning

The OpenFeature provider is versioned independently from the core SDK. The core SDK dependency version is pinned in `openfeature/go.mod` — update it when the provider needs features from a newer core SDK release.

## How users install it

```bash
go get github.com/mixpanel/mixpanel-go/v2/openfeature
```

Because this directory has its own `go.mod`, Go treats it as a separate module. The core SDK is pulled in as a transitive dependency.
