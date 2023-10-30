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
	"github.com/stretchr/testify/require"
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
			assert.Len(t, st.Details(), test.expectedDetailsCount, "want 2, got = %d", len(st.Details()))
			gotResourceInfo := false
			for _, detail := range st.Details() {
				switch detail.(type) {
				case *errdetails.ResourceInfo:
					gotResourceInfo = true
				}
			}
			assert.Equal(t, test.expectedResourceInfo, gotResourceInfo, "expected ResourceInfo, but got none")
			require.EqualError(t, test.expectedError, test.de.Error())
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
