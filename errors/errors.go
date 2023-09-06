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
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/runtime/protoiface"
)

const (
	ErrorCodesFeatureMetadataKey = "error_codes_feature"
	Owner                        = "components-contrib"
	Domain                       = "dapr.io"
	unknown                      = "UNKNOWN_REASON"
)

const (
	// gRPC to HTTP Mapping: 500 Internal Server Error
	unknownHTTPCode = http.StatusInternalServerError
)

var UnknownErrorReason = WithErrorReason(unknown, unknownHTTPCode, codes.Unknown)

type ResourceInfo struct {
	Type string
	Name string
}

// call this function to apply option
type Option func(*Error)

type Error struct {
	err            error
	description    string
	reason         string
	httpCode       int
	grpcStatusCode codes.Code
	metadata       map[string]string
	resourceInfo   *ResourceInfo
}

// EnableComponentErrorCode stores an indicator for
// components to determine if the ErrorCodes Feature
// is enable.
func EnableComponentErrorCode(metadata map[string]string) {
	if metadata != nil {
		metadata[ErrorCodesFeatureMetadataKey] = "true"
	}
}

func featureEnabled(metadata map[string]string) bool {
	_, ok := metadata[ErrorCodesFeatureMetadataKey]
	return ok
}

// New create a new Error using the supplied metadata and Options
// **Note**: As this code is in `Feature Preview`, it will only continue processing
// if the ErrorCodes is enabled
// TODO: @robertojrojas update when feature is ready.
func New(err error, metadata map[string]string, options ...Option) *Error {
	// The following condition can be removed once the
	// Error Codes Feature is GA
	if !featureEnabled(metadata) {
		return nil
	}

	if err == nil {
		return nil
	}

	// Use default values
	de := &Error{
		err:            err,
		reason:         unknown,
		httpCode:       unknownHTTPCode,
		grpcStatusCode: codes.Unknown,
	}

	// Now apply any requested options
	// to override
	for _, option := range options {
		option(de)
	}

	return de
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e != nil && e.err != nil {
		return e.err.Error()
	}
	return ""
}

// Unwrap implements the error unwrapping interface.
func (e *Error) Unwrap() error {
	return e.err
}

// Description returns the description of the error.
func (e *Error) Description() string {
	if e.description != "" {
		return e.description
	}
	return e.err.Error()
}

func WithErrorReason(reason string, httpCode int, grpcStatusCode codes.Code) Option {
	return func(err *Error) {
		err.reason = reason
		err.grpcStatusCode = grpcStatusCode
		err.httpCode = httpCode
	}
}

func WithResourceInfo(resourceInfo *ResourceInfo) Option {
	f := func(e *Error) {
		e.resourceInfo = resourceInfo
	}
	return f
}

func WithDescription(description string) Option {
	f := func(e *Error) {
		e.description = description
	}
	return f
}

func WithMetadata(md map[string]string) Option {
	f := func(e *Error) {
		e.metadata = md
	}
	return f
}

func newErrorInfo(reason string, md map[string]string) *errdetails.ErrorInfo {
	ei := errdetails.ErrorInfo{
		Domain:   Domain,
		Reason:   reason,
		Metadata: md,
	}

	return &ei
}

func newResourceInfo(rid *ResourceInfo, err error) *errdetails.ResourceInfo {
	return &errdetails.ResourceInfo{
		ResourceType: rid.Type,
		ResourceName: rid.Name,
		Owner:        Owner,
		Description:  err.Error(),
	}
}

// *** GRPC Methods ***

// GRPCStatus returns the gRPC status.Status object.
func (e *Error) GRPCStatus() *status.Status {
	messages := []protoiface.MessageV1{
		newErrorInfo(e.reason, e.metadata),
	}

	if e.resourceInfo != nil {
		messages = append(messages, newResourceInfo(e.resourceInfo, e.err))
	}

	ste, stErr := status.New(e.grpcStatusCode, e.description).WithDetails(messages...)
	if stErr != nil {
		return status.New(e.grpcStatusCode, e.description)
	}

	return ste
}

// *** HTTP Methods ***

// ToHTTP transforms the supplied error into
// a GRPC Status and then Marshals it to JSON.
// It assumes if the supplied error is of type Error.
// Otherwise, returns the original error.
func (e *Error) ToHTTP() (int, []byte) {
	if resp, sej := protojson.Marshal(e.GRPCStatus().Proto()); sej == nil {
		return e.httpCode, resp
	}
	return http.StatusInternalServerError, nil
}

// HTTPCode returns the value of the HTTPCode property.
func (e *Error) HTTPCode() int {
	if e.httpCode == 0 {
		return http.StatusInternalServerError
	}
	return e.httpCode
}

// JSONErrorValue implements the errorResponseValue interface.
func (e *Error) JSONErrorValue() []byte {
	b, _ := json.Marshal(e.GRPCStatus().Proto())
	return b
}
