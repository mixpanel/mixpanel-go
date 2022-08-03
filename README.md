#  Mixpanel Go SDK 

## Getting Started

```go
    	mp := NewClient(<PROJECT_ID>, "<PROJECT_TOKEN", "<PROJECT_API_SECRET>")

        // Add a service account 
        mp := NewClient(<PROJECT_ID>, "<PROJECT_TOKEN", "<PROJECT_API_SECRET>", SetServiceAccount("username", "secret"))

        // Setup up for EU Residency
        mp := NewClient(<PROJECT_ID>, "<PROJECT_TOKEN", "<PROJECT_API_SECRET>",EuResidency())
      
```
