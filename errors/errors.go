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
	"fmt"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/dapr/kit/grpccodes"
)

const (
	resourceInfoDefaultOwner = "dapr-components"
	errorInfoDefaultDomain   = "dapr.io"
	errorInfoResonUnknown    = "UNKNOWN_REASON"
)

var UnknownErrorReason = WithErrorReason(errorInfoResonUnknown, codes.Unknown)

// ResourceInfo is meant to be used by Dapr components
// to indicate the Type and Name.
type ResourceInfo struct {
	Type  string
	Name  string
	Owner string
}

// Option allows passing additional information
// to the Error struct.
// See With* functions for further details.
type Option func(*Error)

// Error encapsulates error information
// with additional details like:
//   - http code
//   - grpcStatus code
//   - error reason
//   - metadata information
//   - optional resourceInfo (componenttype/name)
type Error struct {
	err            error
	description    string
	reason         string
	httpCode       int
	grpcStatusCode codes.Code
	metadata       map[string]string
	resourceInfo   *ResourceInfo
}

// New create a new Error using the supplied metadata and Options
func New(err error, metadata map[string]string, options ...Option) *Error {
	if err == nil {
		return nil
	}

	// Use default values
	de := &Error{
		err:            err,
		reason:         errorInfoResonUnknown,
		httpCode:       grpccodes.HTTPStatusFromCode(codes.Unknown),
		grpcStatusCode: codes.Unknown,
		metadata:       metadata,
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
	if e == nil {
		return nil
	}
	return e.err
}

// Description returns the description of the error.
func (e *Error) Description() string {
	if e == nil {
		return ""
	}
	if e.description != "" {
		return e.description
	}
	return e.err.Error()
}

// WithErrorReason used to pass reason and
// grpcStatus code to the Error struct.
func WithErrorReason(reason string, grpcStatusCode codes.Code) Option {
	return func(err *Error) {
		err.reason = reason
		err.grpcStatusCode = grpcStatusCode
		err.httpCode = grpccodes.HTTPStatusFromCode(grpcStatusCode)
	}
}

// WithResourceInfo used to pass ResourceInfo to the Error struct.
func WithResourceInfo(resourceInfo *ResourceInfo) Option {
	return func(e *Error) {
		e.resourceInfo = resourceInfo
	}
}

// WithDescription used to pass a description
// to the Error struct.
func WithDescription(description string) Option {
	return func(e *Error) {
		e.description = description
	}
}

// WithMetadata used to pass a Metadata[string]string
// to the Error struct.
func WithMetadata(md map[string]string) Option {
	return func(e *Error) {
		e.metadata = md
	}
}

func newErrorInfo(reason string, md map[string]string) *errdetails.ErrorInfo {
	return &errdetails.ErrorInfo{
		Domain:   errorInfoDefaultDomain,
		Reason:   reason,
		Metadata: md,
	}
}

func newResourceInfo(rid *ResourceInfo, err error) *errdetails.ResourceInfo {
	owner := resourceInfoDefaultOwner
	if rid.Owner != "" {
		owner = rid.Owner
	}
	return &errdetails.ResourceInfo{
		ResourceType: rid.Type,
		ResourceName: rid.Name,
		Owner:        owner,
		Description:  err.Error(),
	}
}

// *** GRPC Methods ***

// GRPCStatus returns the gRPC status.Status object.
func (e *Error) GRPCStatus() *status.Status {
	var stErr error
	ste := status.New(e.grpcStatusCode, e.description)
	if e.resourceInfo != nil {
		ste, stErr = ste.WithDetails(newErrorInfo(e.reason, e.metadata), newResourceInfo(e.resourceInfo, e.err))
	} else {
		ste, stErr = ste.WithDetails(newErrorInfo(e.reason, e.metadata))
	}
	if stErr != nil {
		return status.New(codes.Internal, fmt.Sprintf("failed to create gRPC status message: %v", stErr))
	}

	return ste
}

// *** HTTP Methods ***

// ToHTTP transforms the supplied error into
// a GRPC Status and then Marshals it to JSON.
// It assumes if the supplied error is of type Error.
// Otherwise, returns the original error.
func (e *Error) ToHTTP() (int, []byte) {
	resp, err := protojson.Marshal(e.GRPCStatus().Proto())
	if err != nil {
		errJSON, _ := json.Marshal(fmt.Sprintf("failed to encode proto to JSON: %v", err))
		return http.StatusInternalServerError, errJSON
	}

	return e.httpCode, resp
}

// HTTPCode returns the value of the HTTPCode property.
func (e *Error) HTTPCode() int {
	if e == nil {
		return http.StatusOK
	}

	return e.httpCode
}

// JSONErrorValue implements the errorResponseValue interface (used by `github.com/dapr/dapr/pkg/http`).
func (e *Error) JSONErrorValue() []byte {
	b, err := protojson.Marshal(e.GRPCStatus().Proto())
	if err != nil {
		errJSON, _ := json.Marshal(fmt.Sprintf("failed to encode proto to JSON: %v", err))
		return errJSON
	}
	return b
}
