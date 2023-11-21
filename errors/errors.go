/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/dapr/kit/logger"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoiface"
)

const (
	domain = "dapr.io"

	errStringFormat = "api error: code = %s desc = %s"
)

var (
	log = logger.NewLogger("dapr.kit")
)

// Error implements the Error interface and the interface that complies with "google.golang.org/grpc/status".FromError().
// It can be used to send errors to HTTP and gRPC servers, indicating the correct status code for each.
type Error struct {
	// Added error details. To see available details see:
	// https://github.com/googleapis/googleapis/blob/master/google/rpc/error_details.proto
	Details []proto.Message

	// Status code for gRPC responses.
	GrpcCode grpcCodes.Code

	// Status code for HTTP responses.
	HttpCode int

	// Message is the human-readable error message.
	Message string

	// Tag is a string identifying the error, used with HTTP responses only.
	Tag string
}

// New create a new Error using the supplied metadata and Options
func New(grpcCode grpcCodes.Code, httpCode int, message string, tag string) *Error {
	// Use default values
	kitError := &Error{
		Details:  make([]proto.Message, 0),
		GrpcCode: grpcCode,
		HttpCode: httpCode,
		Message:  message,
		Tag:      tag,
	}

	return kitError
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e != nil {
		return e.String()
	}
	return ""
}

// String returns the string representation, useful for debugging.
func (e Error) String() string {
	return fmt.Sprintf(errStringFormat, e.GrpcCode.String(), e.Message)
}

// WithResourceInfo used to pass ResourceInfo to the Error struct.
func (e *Error) WithResourceInfo(resourceType string, resourceName string, owner string, description string) *Error {
	resourceInfo := &errdetails.ResourceInfo{
		ResourceType: resourceType,
		ResourceName: resourceName,
		Owner:        owner,
		Description:  description,
	}

	e.Details = append(e.Details, resourceInfo)

	return e
}

// WithErrorInfo adds error information to the Error struct.
func (e *Error) WithErrorInfo(reason string, metadata map[string]string) *Error {
	errorInfo := &errdetails.ErrorInfo{
		Domain:   domain,
		Reason:   reason,
		Metadata: metadata,
	}
	e.Details = append(e.Details, errorInfo)

	return e
}

// WithVars returns a copy of the error with the message going through fmt.Sprintf with the arguments passed to this method.
func (e *Error) WithVars(a ...any) *Error {
	eCopy := *e
	eCopy.Message = fmt.Sprintf(e.Message, a...)

	return &eCopy
}

func (e *Error) WithDetails(details ...proto.Message) *Error {
	e.Details = append(e.Details, details...)

	return e

	//TODO ensure proto.message in Details is actually a correct proto type
	//TODO add test for it
}

// *** GRPC Methods ***

// GRPCStatus returns the gRPC status.Status object.
func (e *Error) GRPCStatus() *status.Status {
	stat := status.New(e.GrpcCode, e.Message)

	// convert details from proto.Msg -> protoiface.MsgV1
	var convertedDetails []protoiface.MessageV1
	for _, detail := range e.Details {
		if v1, ok := detail.(protoiface.MessageV1); ok {
			convertedDetails = append(convertedDetails, v1)
		} else {
			log.Debugf("Failed to convert error details: %s", detail)
		}
	}

	if len(e.Details) > 0 {
		var err error
		stat, err = stat.WithDetails(convertedDetails...)
		if err != nil {
			log.Debugf("Failed to add error details: %s to status: %s", err, stat)
		}
	}

	return stat

}

// *** HTTP Methods ***

// JSONErrorValue implements the errorResponseValue interface.
func (e *Error) JSONErrorValue() []byte {
	grpcStatus := e.GRPCStatus().Proto()

	// Make httpCode human readable
	httpStatus := http.StatusText(e.HttpCode)

	// Change output to match prior output to not break users
	errJson := map[string]interface{}{
		"errorCode": httpStatus,
		"message":   grpcStatus.GetMessage(),
	}

	// Handle err details
	details := e.Details
	if len(details) > 0 {
		errJson["details"] = make([]interface{}, len(details))
		for i, detail := range details {
			// cast to interface to be able to do type switch
			// over all possible error_details defined
			// https://github.com/googleapis/go-genproto/blob/main/googleapis/rpc/errdetails/error_details.pb.go
			switch typedDetail := detail.(type) {
			case *errdetails.ErrorInfo:
				detailMap := map[string]interface{}{
					"reason":   typedDetail.Reason,
					"domain":   typedDetail.Domain,
					"metadata": typedDetail.Metadata, //TODO: fix this
				}
				errJson["details"].([]interface{})[i] = detailMap
			case *errdetails.RetryInfo:
			case *errdetails.DebugInfo:
			case *errdetails.QuotaFailure:
			case *errdetails.PreconditionFailure:
			case *errdetails.BadRequest:
			case *errdetails.RequestInfo:
			case *errdetails.ResourceInfo:
				detailMap := map[string]interface{}{
					"resourceType": typedDetail.ResourceType,
					"resourceName": typedDetail.ResourceName,
					"owner":        typedDetail.Owner,
					"description":  typedDetail.Description, //TODO: fix this not being set if description is err.Message
				}
				errJson["details"].([]interface{})[i] = detailMap
			case *errdetails.Help:
			case *errdetails.LocalizedMessage:
			case *errdetails.QuotaFailure_Violation:
			case *errdetails.PreconditionFailure_Violation:
			case *errdetails.BadRequest_FieldViolation:
			case *errdetails.Help_Link:

			default:
				log.Debugf("Failed to convert error details due to incorrect type. \nSee types here: https://github.com/googleapis/googleapis/blob/master/google/rpc/error_details.proto. \nDetail: %s", detail)
				// Handle unknown detail types
				unknownDetail := map[string]interface{}{
					"unknownDetailType": fmt.Sprintf("%T", typedDetail),
				}
				errJson["details"].([]interface{})[i] = unknownDetail
			}
		}
	}

	errBytes, err := json.Marshal(errJson)
	if err != nil {
		errJSON, _ := json.Marshal(fmt.Sprintf("failed to encode proto to JSON: %v", err))
		return errJSON
	}
	return errBytes
}

// Is implements the interface that checks if the error matches the given one.
func (e *Error) Is(targetI error) bool {
	// Ignore the message in the comparison because the target could have been formatted
	var target *Error
	if !errors.As(targetI, &target) {
		return false
	}
	return e.Tag == target.Tag &&
		e.GrpcCode == target.GrpcCode &&
		e.HttpCode == target.HttpCode
}
