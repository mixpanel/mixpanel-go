module github.com/mixpanel/mixpanel-go/openfeature

go 1.22

require (
	// v2.1.0+ ships VariantSource + FallbackReason on SelectedVariant (SDK-79)
	// which provider.go now dispatches on. Earlier versions lack those fields
	// and would silently degrade to the pre-SDK-79 behavior.
	github.com/mixpanel/mixpanel-go/v2 v2.1.0
	github.com/open-feature/go-sdk v1.13.0
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/barkimedes/go-deepcopy v0.0.0-20220514131651-17c30cfc62df // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/diegoholiveira/jsonlogic/v3 v3.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
