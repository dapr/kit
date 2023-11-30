# Errors

The standardizing of errors to be used in Dapr based on the gRPC Richer Error Model and [accepted dapr/proposal](https://github.com/dapr/proposals/blob/main/0009-BCIRS-error-handling-codes.md).

## Usage

```go
import kitErrors "github.com/dapr/kit/errors"

// Define error in dapr pkg/api/errors.go
ErrPubSubNotFound = kitErrors.New(
    grpcCodes.NotFound,
    http.StatusBadRequest,
    "pubsub %s is not found",
    fmt.Sprintf("%s%s", PubSub, ErrNotFound),

// Use error in dapr
err = errutil.ErrPubSubNotFound.WithVars(pubsubName)
err = err.WithErrorInfo(err.Message, reqMeta).
WithResourceInfo(pubsubType, pubsubName, "", err.Message)
```
