#  Mixpanel Go SDK

[![Go](https://github.com/mixpanel/mixpanel-go/actions/workflows/testing.yaml/badge.svg)](https://github.com/mixpanel/mixpanel-go/actions/workflows/testing.yaml)
[![codecov](https://codecov.io/gh/mixpanel/mixpanel-go/branch/main/graph/badge.svg?token=SRZPEYRHEU)](https://codecov.io/gh/mixpanel/mixpanel-go)

## Getting Started

```go
    	mp := NewClient(<PROJECT_ID>, "<PROJECT_TOKEN", "<PROJECT_API_SECRET>")

        // Add a service account 
        mp := NewClient(<PROJECT_ID>, "<PROJECT_TOKEN", "<PROJECT_API_SECRET>", SetServiceAccount("username", "secret"))

        // Setup up for EU Residency
        mp := NewClient(<PROJECT_ID>, "<PROJECT_TOKEN", "<PROJECT_API_SECRET>",EuResidency())
      
```
