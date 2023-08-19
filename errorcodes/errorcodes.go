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

package errorcodes

import (
	"errors"

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
	NoReason                     = "NO_REASON_SPECIFIED"
)

var NoErrorReason = WithErrorReason(NoReason, 503, codes.Internal)

type ResourceInfo struct {
	ResourceType string
	ResourceName string
}

func WithErrorReason(reason string, httpCode int, grpcStatusCode codes.Code) ErrorOption {
	f := func(er *DaprError) {
		er.reason = reason
		er.grpcStatusCode = grpcStatusCode
		er.httpCode = httpCode
	}
	return f
}

// call this function to apply option
type ErrorOption func(*DaprError)

type DaprError struct {
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
	if _, ok := metadata[ErrorCodesFeatureMetadataKey]; ok {
		return true
	}
	return false
}

// NewDaprError create a new DaprError using the supplied metadata and ErrorOptions
// **Note**: As this code is in `Feature Preview`, it will only continue processing
// if the ErrorCodes is enabled
func NewDaprError(err error, metadata map[string]string, options ...ErrorOption) *DaprError {
	// The following condition can be removed once the
	// Error Codes Feature is GA
	if !featureEnabled(metadata) {
		return nil
	}

	if err == nil {
		return nil
	}

	// Use default values
	de := &DaprError{
		err:            err,
		description:    err.Error(),
		reason:         NoReason,
		httpCode:       503,
		grpcStatusCode: codes.Internal,
	}

	// Now apply any requested options
	// to overr
	for _, option := range options {
		option(de)
	}

	return de
}

// Error implements the error interface.
func (c *DaprError) Error() string {
	if c.err != nil {
		return c.err.Error()
	}
	return ""
}

// Unwrap implements the error unwrapping interface.
func (c *DaprError) Unwrap() error {
	return c.err
}

// Description returns the description of the error.
func (c *DaprError) Description() string {
	if c.description != "" {
		return c.description
	}
	return c.err.Error()
}

func (c *DaprError) SetDescription(description string) {
	c.description = description
}

func (c *DaprError) SetResourceInfoData(resourceInfoData *ResourceInfo) {
	c.resourceInfo = resourceInfoData
}

func (c *DaprError) SetMetadata(md map[string]string) {
	c.metadata = md
}

func WithResourceInfo(resourceInfoData *ResourceInfo) ErrorOption {
	f := func(de *DaprError) {
		de.resourceInfo = resourceInfoData
	}
	return f
}

func WithDescription(description string) ErrorOption {
	f := func(de *DaprError) {
		de.description = description
	}
	return f
}

func WithMetadata(md map[string]string) ErrorOption {
	f := func(de *DaprError) {
		de.metadata = md
	}
	return f
}

// *** GRPC Methods ***

// FromDaprErrorToGRPC transforms the supplied error into
// a GRPC Status. It assumes if the supplied error
// is of type DaprError.
// Otherwise, returns the original error.
func FromDaprErrorToGRPC(err error) (*status.Status, error) {
	de := &DaprError{}
	if errors.As(err, &de) {
		if st, ese := de.newStatusError(); ese == nil {
			return st, nil
		}
		return nil, err
	}

	return nil, err
}

func (c *DaprError) newStatusError() (*status.Status, error) {
	messages := []protoiface.MessageV1{
		newErrorInfo(c.reason, c.metadata),
	}

	if c.resourceInfo != nil {
		messages = append(messages, newResourceInfo(c.resourceInfo, c.err))
	}

	ste, stErr := status.New(c.grpcStatusCode, c.description).WithDetails(messages...)
	if stErr != nil {
		return nil, stErr
	}

	return ste, nil
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
		ResourceType: rid.ResourceType,
		ResourceName: rid.ResourceName,
		Owner:        Owner,
		Description:  err.Error(),
	}
}

// *** HTTP Methods ***

// FromDaprErrorToHTTP transforms the supplied error into
// a GRPC Status and then Marshals it to JSON.
// It assumes if the supplied error is of type DaprError.
// Otherwise, returns the original error.
func FromDaprErrorToHTTP(err error) (int, []byte, error) {
	de := &DaprError{}
	if errors.As(err, &de) {
		if st, ese := de.newStatusError(); ese == nil {
			resp, sej := statusErrorJSON(st)
			if sej != nil {
				return 0, nil, err
			}

			return de.httpCode, resp, nil
		}
	}
	return 0, nil, err
}

func statusErrorJSON(st *status.Status) ([]byte, error) {
	return protojson.Marshal(st.Proto())
}
