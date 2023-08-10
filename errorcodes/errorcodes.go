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
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/runtime/protoiface"
)

type Reason string

const (
	ErrorCodesFeatureMetadataKey = "error_codes_feature"
	Owner                        = "components-contrib"
	Domain                       = "dapr.io"
)

var (
	NoReason                       = Reason("NO_REASON_SPECIFIED")
	StateETagMismatchReason        = Reason("DAPR_STATE_ETAG_MISMATCH")
	StateETagInvalidReason         = Reason("DAPR_STATE_ETAG_INVALID")
	TopicNotFoundReason            = Reason("DAPR_TOPIC_NOT_FOUND")
	SecretKeyNotFoundReason        = Reason("DAPR_SECRET_KEY_NOT_FOUND")
	ConfigurationKeyNotFoundReason = Reason("DAPR_CONFIG_KEY_NOT_FOUND")
)

type ResourceInfoData struct {
	ResourceType string
	ResourceName string
}

// call this function to apply option
type ErrorOption func(*DaprError)

type DaprError struct {
	err              error
	description      string
	reason           Reason
	resourceInfoData *ResourceInfoData
	metadata         map[string]string
}

func ActivateErrorCodesFeature(enabled bool, metadata map[string]string) {
	if enabled && metadata != nil {
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

	de := &DaprError{
		err:         err,
		description: err.Error(),
		reason:      NoReason,
	}

	// Now apply any requested options
	for _, option := range options {
		option(de)
	}

	return de
}

// Error implements the error interface.
func (c DaprError) Error() string {
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

func (c *DaprError) SetResourceInfoData(resourceInfoData *ResourceInfoData) {
	c.resourceInfoData = resourceInfoData
}

func (c *DaprError) SetMetadata(md map[string]string) {
	c.metadata = md
}

func WithReason(reason Reason) ErrorOption {
	f := func(de *DaprError) {
		de.reason = reason
	}
	return f
}

func WithResourceInfoData(resourceInfoData *ResourceInfoData) ErrorOption {
	f := func(de *DaprError) {
		de.resourceInfoData = resourceInfoData
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
// is of type DaprError. Otherwise, an error will be returned
// instead.
func FromDaprErrorToGRPC(err error) (*status.Status, error) {
	de := &DaprError{}
	var st *status.Status
	var ese error
	if errors.As(err, de) {
		st, ese = newStatusError(de)
		if ese != nil {
			return nil, ese
		}
		return st, nil
	}

	return nil, fmt.Errorf("unable to a DaprError from: %v", err)
}

func newStatusError(de *DaprError) (*status.Status, error) {
	cd := convertReasonToStatusCode(de.reason)
	messages := []protoiface.MessageV1{
		newErrorInfo(de.reason, de.metadata),
	}

	if de.resourceInfoData != nil {
		messages = append(messages, newResourceInfo(de.resourceInfoData, de.err))
	}

	ste, stErr := status.New(cd, de.description).WithDetails(messages...)
	if stErr != nil {
		return nil, stErr
	}

	return ste, nil
}

func newErrorInfo(reason Reason, md map[string]string) *errdetails.ErrorInfo {
	ei := errdetails.ErrorInfo{
		Domain:   Domain,
		Reason:   string(reason),
		Metadata: md,
	}

	return &ei
}

func newResourceInfo(rid *ResourceInfoData, err error) *errdetails.ResourceInfo {
	return &errdetails.ResourceInfo{
		ResourceType: rid.ResourceType,
		ResourceName: rid.ResourceName,
		Owner:        Owner,
		Description:  err.Error(),
	}
}

func statusErrorJSON(st *status.Status) ([]byte, error) {
	return protojson.Marshal(st.Proto())
}

func convertReasonToStatusCode(reason Reason) codes.Code {
	c := codes.Aborted
	switch reason {
	case StateETagMismatchReason:
		c = codes.Aborted
	case StateETagInvalidReason:
		c = codes.InvalidArgument
	case TopicNotFoundReason:
		c = codes.NotFound
	case SecretKeyNotFoundReason:
		c = codes.NotFound
	case ConfigurationKeyNotFoundReason:
		c = codes.NotFound
	}

	return c
}

// *** HTTP Methods ***

// FromDaprErrorToHTTP transforms the supplied error into
// a GRPC Status. It assumes if the supplied error
// is of type DaprError. Otherwise, an error will be returned
// instead.
func FromDaprErrorToHTTP(err error) (int, []byte, error) {
	st, fe := FromDaprErrorToGRPC(err)
	if fe != nil {
		return 0, nil, fe
	}

	httpCode := convertStatusCodeToHTTPCode(st.Code())
	resp, sej := statusErrorJSON(st)
	if sej != nil {
		return 0, nil, sej
	}

	return httpCode, resp, nil
}

func convertStatusCodeToHTTPCode(code codes.Code) int {
	c := 503
	switch code {
	case codes.Aborted:
		c = 409
	case codes.InvalidArgument:
		c = 400
	case codes.NotFound:
		c = 404
	}

	return c
}
