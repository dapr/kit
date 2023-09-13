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

package grpccodes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

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
