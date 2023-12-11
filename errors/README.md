# Errors

The standardizing of errors to be used in Dapr based on the gRPC Richer Error Model and [accepted dapr/proposal](https://github.com/dapr/proposals/blob/main/0009-BCIRS-error-handling-codes.md).

## Usage

```go
import kitErrors "github.com/dapr/kit/errors"

// Define error in dapr pkg/api/<building_block>_errors.go
func PubSubNotFound(pubsubName string, pubsubType string, metadata map[string]string) error {
message := fmt.Sprintf("pubsub %s is not found", pubsubName)

return kitErrors.New(
grpcCodes.NotFound,
http.StatusBadRequest,
message,
fmt.Sprintf("%s%s", kitErrors.CodePrefixPubSub, kitErrors.CodeNotFound),
).
WithErrorInfo(kitErrors.CodePrefixPubSub+kitErrors.CodeNotFound, metadata).
WithResourceInfo(pubsubType, pubsubName, "", message)
}

// Use error in dapr and pass in relevant information
err = errutil.PubSubNotFound(pubsubName, pubsubType, metadata)

```
