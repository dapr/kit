# Errors

The standardizing of errors to be used in Dapr based on the gRPC Richer Error Model and [accepted dapr/proposal](https://github.com/dapr/proposals/blob/main/0009-BCIRS-error-handling-codes.md).

## Usage

Define the error
```go
import kitErrors "github.com/dapr/kit/errors"

// Define error in dapr pkg/api/errors/<building_block>.go
func PubSubNotFound(name string, pubsubType string, metadata map[string]string) error {
	message := fmt.Sprintf("pubsub %s is not found", name)

	return kitErrors.NewBuilder(
		grpcCodes.NotFound,
		http.StatusBadRequest,
		message,
		kitErrors.CodePrefixPubSub+kitErrors.CodeNotFound,
	).
		WithErrorInfo(kitErrors.CodePrefixPubSub+kitErrors.CodeNotFound, metadata).
		WithResourceInfo(pubsubType, name, "", message).
		Build()
}
```

Use the error
```go
import apiErrors "github.com/dapr/dapr/pkg/api/errors"

// Use error in dapr and pass in relevant information
err = apiErrors.PubSubNotFound(pubsubName, pubsubType, metadata)

```
