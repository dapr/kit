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
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
)

func TestNewErrorReason(t *testing.T) {
	tests := []struct {
		name              string
		inErr             error
		inMetadata        map[string]string
		inOptions         []Option
		expectedDaprError bool
		expectedReason    string
	}{
		{
			name:              "Error_New_OK_No_Reason",
			inErr:             &Error{},
			inMetadata:        map[string]string{},
			expectedDaprError: true,
			expectedReason:    "UNKNOWN_REASON",
		},
		{
			name:              "DaprError_New_OK_StateETagMismatchReason",
			inErr:             &Error{},
			inMetadata:        map[string]string{},
			expectedDaprError: true,
			inOptions: []Option{
				WithErrorReason("StateETagMismatchReason", codes.NotFound),
			},
			expectedReason: "StateETagMismatchReason",
		},
		{
			name:              "DaprError_New_OK_StateETagInvalidReason",
			inErr:             &Error{},
			inMetadata:        map[string]string{},
			expectedDaprError: true,
			inOptions: []Option{
				WithErrorReason("StateETagInvalidReason", codes.Aborted),
			},
			expectedReason: "StateETagInvalidReason",
		},
		{
			name:              "DaprError_New_Nil_Error",
			inErr:             nil,
			inMetadata:        map[string]string{},
			expectedDaprError: false,
		},
		{
			name:              "DaprError_New_Nil_Metadata",
			inErr:             nil,
			expectedDaprError: false,
		},
		{
			name:              "DaprError_New_Metadata_No_ErrorCodes_Key",
			inErr:             nil,
			inMetadata:        map[string]string{},
			expectedDaprError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			de := New(test.inErr, test.inMetadata, test.inOptions...)
			if test.expectedDaprError {
				assert.NotNil(t, de, "expected DaprError but got none")
				assert.Equal(t, test.expectedReason, de.reason, "want %s, but got = %v", test.expectedReason, de.reason)
			} else {
				assert.Nil(t, de, "unexpected DaprError but got %v", de)
			}
		})
	}
}

func TestNewError(t *testing.T) {
	md := map[string]string{}
	tests := []struct {
		name                 string
		de                   *Error
		expectedDetailsCount int
		expectedResourceInfo bool
		expectedError        error
		expectedDescription  string
	}{
		{
			name: "WithResourceInfo_OK",
			de: New(fmt.Errorf("some error"), md,
				WithResourceInfo(&ResourceInfo{Type: "testResourceType", Name: "testResourceName"})),
			expectedDetailsCount: 2,
			expectedResourceInfo: true,
			expectedError:        fmt.Errorf("some error"),
			expectedDescription:  "some error",
		},
		{
			name:                 "ResourceInfo_Empty",
			de:                   New(fmt.Errorf("some error"), md, WithDescription("some"), WithErrorReason("StateETagInvalidReason", codes.Aborted)),
			expectedDetailsCount: 1,
			expectedResourceInfo: false,
			expectedError:        fmt.Errorf("some error"),
			expectedDescription:  "some",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st := test.de.GRPCStatus()
			assert.NotNil(t, st, "want nil, got = %v", st)
			assert.NotNil(t, st.Details())
			assert.Equal(t, test.expectedDetailsCount, len(st.Details()), "want 2, got = %d", len(st.Details()))
			gotResourceInfo := false
			for _, detail := range st.Details() {
				switch detail.(type) {
				case *errdetails.ResourceInfo:
					gotResourceInfo = true
				}
			}
			assert.Equal(t, test.expectedResourceInfo, gotResourceInfo, "expected ResourceInfo, but got none")
			assert.EqualError(t, test.expectedError, test.de.Error())
			assert.Equal(t, test.expectedDescription, test.de.Description())
		})
	}
}

func TestToHTTP(t *testing.T) {
	md := map[string]string{}
	tests := []struct {
		name                 string
		de                   *Error
		expectedCode         int
		expectedReason       string
		expectedResourceType string
		expectedResourceName string
	}{
		{
			name: "WithResourceInfo_OK",
			de: New(fmt.Errorf("some error"), md,
				WithResourceInfo(&ResourceInfo{Type: "testResourceType", Name: "testResourceName"})),
			expectedCode:         http.StatusInternalServerError,
			expectedReason:       "UNKNOWN_REASON",
			expectedResourceType: "testResourceType",
			expectedResourceName: "testResourceName",
		},
		{
			name: "WithResourceInfo_OK",
			de: New(fmt.Errorf("some error"), md,
				WithErrorReason("RedisFailure", codes.Internal),
				WithResourceInfo(&ResourceInfo{Type: "testResourceType", Name: "testResourceName"})),
			expectedCode:         http.StatusInternalServerError,
			expectedReason:       "RedisFailure",
			expectedResourceType: "testResourceType",
			expectedResourceName: "testResourceName",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			i, b := test.de.ToHTTP()
			bodyStr := string(b)
			assert.Equal(t, test.expectedCode, i, "want %d, got = %d", test.expectedCode, i)
			assert.Contains(t, bodyStr, test.expectedReason)
			assert.Contains(t, bodyStr, test.expectedResourceName)
			assert.Contains(t, bodyStr, test.expectedResourceType)
		})
	}
}

func TestGRPCStatus(t *testing.T) {
	md := map[string]string{}
	tests := []struct {
		name          string
		de            *Error
		expectedCode  int
		expectedBytes int
		expectedJSON  string
	}{
		{
			name: "WithResourceInfo_OK",
			de: New(fmt.Errorf("some error"), md,
				WithResourceInfo(&ResourceInfo{Type: "testResourceType", Name: "testResourceName"}), WithMetadata(md)),
			expectedCode:  500,
			expectedBytes: 308,
			expectedJSON:  `{"code":2, "details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo", "reason":"UNKNOWN_REASON", "domain":"dapr.io"}, {"@type":"type.googleapis.com/google.rpc.ResourceInfo", "resourceType":"testResourceType", "resourceName":"testResourceName", "owner":"components-contrib", "description":"some error"}]}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.de.GRPCStatus()
			// assert.NotNil(t, st, i, "want %d, got = %d", test.expectedCode, i)
			// assert.Equal(t, test.expectedBytes, len(b), "want  %d bytes, got = %d", test.expectedBytes, len(b))
			// assert.Equal(t, test.expectedJSON, string(b), "want JSON %s , got = %s", test.expectedJSON, string(b))
		})
	}
}

func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		name   string
		code   codes.Code
		result int
	}{
		{
			name:   "codes.OK-http.StatusOK",
			code:   codes.OK,
			result: http.StatusOK,
		},
		{
			name:   "codes.Canceled-http.StatusRequestTimeout",
			code:   codes.Canceled,
			result: http.StatusRequestTimeout,
		},
		{
			name:   "codes.Unknown-http.StatusInternalServerError",
			code:   codes.Unknown,
			result: http.StatusInternalServerError,
		},
		{
			name:   "codes.InvalidArgument-http.StatusBadRequest",
			code:   codes.InvalidArgument,
			result: http.StatusBadRequest,
		},
		{
			name:   "codes.DeadlineExceeded-http.StatusGatewayTimeout",
			code:   codes.DeadlineExceeded,
			result: http.StatusGatewayTimeout,
		},
		{
			name:   "codes.NotFound-http.StatusNotFound",
			code:   codes.NotFound,
			result: http.StatusNotFound,
		},
		{
			name:   "codes.AlreadyExists-http.StatusConflict",
			code:   codes.AlreadyExists,
			result: http.StatusConflict,
		},
		{
			name:   "codes.PermissionDenied-http.StatusForbidden",
			code:   codes.PermissionDenied,
			result: http.StatusForbidden,
		},
		{
			name:   "codes.Unauthenticated-http.StatusUnauthorized",
			code:   codes.Unauthenticated,
			result: http.StatusUnauthorized,
		},
		{
			name:   "codes.ResourceExhausted-http.StatusTooManyRequests",
			code:   codes.ResourceExhausted,
			result: http.StatusTooManyRequests,
		},
		{
			name:   "codes.FailedPrecondition-http.StatusBadRequest",
			code:   codes.FailedPrecondition,
			result: http.StatusBadRequest,
		},
		{
			name:   "codes.Aborted-http.StatusConflict",
			code:   codes.Aborted,
			result: http.StatusConflict,
		},
		{
			name:   "codes.OutOfRange-http.StatusBadRequest",
			code:   codes.OutOfRange,
			result: http.StatusBadRequest,
		},
		{
			name:   "codes.Unimplemented-http.StatusNotImplemented",
			code:   codes.Unimplemented,
			result: http.StatusNotImplemented,
		},
		{
			name:   "codes.Internal-http.StatusInternalServerError",
			code:   codes.Internal,
			result: http.StatusInternalServerError,
		},
		{
			name:   "codes.Unavailable-http.StatusServiceUnavailable",
			code:   codes.Unavailable,
			result: http.StatusServiceUnavailable,
		},
		{
			name:   "codes.DataLoss-http.StatusInternalServerError",
			code:   codes.DataLoss,
			result: http.StatusInternalServerError,
		},
		{
			name:   "codes.InvalidCode-http.StatusInternalServerError",
			code:   57, // codes.Code Does not exist
			result: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := HTTPStatusFromCode(tt.code)
			assert.Equal(t, tt.result, rt)
		})
	}
}

func TestCodeFromHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		httpcode int
		result   codes.Code
	}{
		{
			name:     "http.OK-codes.OK",
			httpcode: http.StatusOK,
			result:   codes.OK,
		},
		{
			name:     "http.StatusCreated-codes.OK",
			httpcode: http.StatusCreated,
			result:   codes.OK,
		},
		{
			name:     "http.StatusAccepted-codes.OK",
			httpcode: http.StatusAccepted,
			result:   codes.OK,
		},
		{
			name:     "http.StatusNonAuthoritativeInfo-codes.OK",
			httpcode: http.StatusNonAuthoritativeInfo,
			result:   codes.OK,
		},
		{
			name:     "http.StatusNoContent-codes.OK",
			httpcode: http.StatusNoContent,
			result:   codes.OK,
		},
		{
			name:     "http.StatusResetContent-codes.OK",
			httpcode: http.StatusResetContent,
			result:   codes.OK,
		},
		{
			name:     "http.StatusPartialContent-codes.OK",
			httpcode: http.StatusPartialContent,
			result:   codes.OK,
		},
		{
			name:     "http.StatusMultiStatus-codes.OK",
			httpcode: http.StatusMultiStatus,
			result:   codes.OK,
		},
		{
			name:     "http.StatusAlreadyReported-codes.OK",
			httpcode: http.StatusAlreadyReported,
			result:   codes.OK,
		},
		{
			name:     "http.StatusIMUsed-codes.OK",
			httpcode: http.StatusOK,
			result:   codes.OK,
		},
		{
			name:     "http.StatusRequestTimeout-codes.Canceled",
			httpcode: http.StatusRequestTimeout,
			result:   codes.Canceled,
		},
		{
			name:     "http.StatusInternalServerError-codes.Unknown",
			httpcode: http.StatusInternalServerError,
			result:   codes.Unknown,
		},
		{
			name:     "http.StatusBadRequest-codes.Internal",
			httpcode: http.StatusBadRequest,
			result:   codes.Internal,
		},
		{
			name:     "http.StatusGatewayTimeout-codes.DeadlineExceeded",
			httpcode: http.StatusGatewayTimeout,
			result:   codes.DeadlineExceeded,
		},
		{
			name:     "http.StatusNotFound-codes.NotFound",
			httpcode: http.StatusNotFound,
			result:   codes.NotFound,
		},
		{
			name:     "http.StatusConflict-codes.AlreadyExists",
			httpcode: http.StatusConflict,
			result:   codes.AlreadyExists,
		},
		{
			name:     "http.StatusForbidden-codes.PermissionDenied",
			httpcode: http.StatusForbidden,
			result:   codes.PermissionDenied,
		},
		{
			name:     "http.StatusUnauthorized-codes.Unauthenticated",
			httpcode: http.StatusUnauthorized,
			result:   codes.Unauthenticated,
		},
		{
			name:     "http.StatusTooManyRequests-codes.ResourceExhausted",
			httpcode: http.StatusTooManyRequests,
			result:   codes.ResourceExhausted,
		},
		{
			name:     "http.StatusNotImplemented-codes.Unimplemented",
			httpcode: http.StatusNotImplemented,
			result:   codes.Unimplemented,
		},
		{
			name:     "http.StatusServiceUnavailable-codes.Unavailable",
			httpcode: http.StatusServiceUnavailable,
			result:   codes.Unavailable,
		},
		{
			name:     "HTTPStatusDoesNotExist-codes.Unavailable",
			httpcode: 999,
			result:   codes.Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := CodeFromHTTPStatus(tt.httpcode)
			assert.Equal(t, tt.result, rt)
		})
	}
}
