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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
)

func TestActivateErrorCodesFeature(t *testing.T) {
	tests := []struct {
		name     string
		md       map[string]string
		enabled  bool
		expected bool
	}{
		{
			name:     "ActivateErrorCodesFeature_OK",
			md:       map[string]string{},
			enabled:  true,
			expected: true,
		},
		{
			name:     "ActivateErrorCodesFeature_Enabled_False",
			md:       map[string]string{},
			enabled:  false,
			expected: false,
		},
		{
			name:     "ActivateErrorCodesFeature_Nil_Map_False",
			md:       nil,
			enabled:  true,
			expected: false,
		},
		{
			name:     "ActivateErrorCodesFeature_Disabled_Nil_Map_False",
			md:       nil,
			enabled:  false,
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.enabled {
				EnableComponentErrorCode(test.md)
			}
			if _, ok := test.md[ErrorCodesFeatureMetadataKey]; ok != test.expected {
				t.Errorf("unexpected result - expected %t, but got %t", ok, test.expected)
			}
		})
	}
}

func TestNewDaprError(t *testing.T) {
	tests := []struct {
		name              string
		inErr             error
		inMetadata        map[string]string
		expectedDaprError bool
	}{
		{
			name:  "DaprError_New_OK_No_Reason",
			inErr: &DaprError{},
			inMetadata: map[string]string{
				ErrorCodesFeatureMetadataKey: "true",
			},
			expectedDaprError: true,
		},
		{
			name:  "DaprError_New_Nil_Error",
			inErr: nil,
			inMetadata: map[string]string{
				ErrorCodesFeatureMetadataKey: "true",
			},
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
			de := NewDaprError(test.inErr, test.inMetadata)
			if test.expectedDaprError {
				assert.NotNil(t, test.expectedDaprError, "expected DaprError but got none")
			} else {
				assert.Nil(t, de, "unexpected DaprError but got %v", de)
			}
		})
	}
}

func TestDaprErrorNewStatusError(t *testing.T) {
	md := map[string]string{
		ErrorCodesFeatureMetadataKey: "true",
	}

	tests := []struct {
		name                 string
		de                   *DaprError
		expectedDetailsCount int
		expectedResourceInfo bool
	}{
		{
			name: "WithResourceInfo_OK",
			de: NewDaprError(fmt.Errorf("some error"), md,
				WithResourceInfo(&ResourceInfo{ResourceType: "testResourceType", ResourceName: "testResourceName"})),
			expectedDetailsCount: 2,
			expectedResourceInfo: true,
		},
		{
			name:                 "ResourceInfo_Empty",
			de:                   NewDaprError(fmt.Errorf("some error"), md),
			expectedDetailsCount: 1,
			expectedResourceInfo: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st, e := test.de.newStatusError()
			assert.Nil(t, e, "want nil, got = %v", e)
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
		})
	}
}

func TestFromDaprErrorToGRPC(t *testing.T) {
	tests := []struct {
		name   string
		errIn  error
		expErr string
	}{
		{
			name:  "FromDaprErrorToGRPC_OK",
			errIn: &DaprError{},
		},
		{
			name:   "FromDaprErrorToGRPC_Nil",
			expErr: "unable to convert to a DaprError from input value: <nil>",
		},
		{
			name:   "FromDaprErrorToGRPC_Invalid",
			errIn:  fmt.Errorf("invalid"),
			expErr: "unable to convert to a DaprError from input value: invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st, e := FromDaprErrorToGRPC(test.errIn)

			if test.expErr == "" {
				assert.NoError(t, e, fmt.Sprintf("wanted nil, but got = %v", e))
				assert.NotNil(t, st, "unexpected nil value for Status")
			} else {
				assert.Error(t, e)
				assert.EqualError(t, e, test.expErr)
			}
		})
	}
}

func TestFromDaprErrorToHTTP(t *testing.T) {
	md := map[string]string{
		ErrorCodesFeatureMetadataKey: "true",
	}
	tests := []struct {
		name    string
		errIn   error
		expErr  string
		expCode int
	}{
		{
			name:    "FromDaprErrorToHTTP_OK",
			errIn:   NewDaprError(fmt.Errorf("503"), md),
			expCode: 503,
		},
		{
			name:   "FromDaprErrorToHTTP_Nil",
			expErr: "unable to convert to a DaprError from input value: <nil>",
		},
		{
			name:   "FromDaprErrorToHTTP_Invalid",
			errIn:  fmt.Errorf("invalid"),
			expErr: "unable to convert to a DaprError from input value: invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, b, e := FromDaprErrorToHTTP(test.errIn)

			if test.expErr == "" {
				assert.NoError(t, e, fmt.Sprintf("wanted nil, but got = %v", e))
				assert.NotNil(t, b, "unexpected nil value for bytes")
				assert.Equal(t, test.expCode, code)
			} else {
				assert.Error(t, e)
				assert.EqualError(t, e, test.expErr)
			}
		})
	}
}

func TestFeatureEnabled(t *testing.T) {
	tests := []struct {
		name     string
		md       map[string]string
		expected bool
	}{
		{
			name: "FeatureEnabled_OK",
			md: map[string]string{
				ErrorCodesFeatureMetadataKey: "true",
			},
			expected: true,
		},
		{
			name:     "FeatureEnabled_NoMap",
			expected: false,
		},
		{
			name: "FeatureEnabled_Map_MissingKey",
			md: map[string]string{
				"other": "1",
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := featureEnabled(test.md)
			assert.Equal(t, test.expected, b)
		})
	}
}

func TestConvertReasonToStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		reason       Reason
		expectedCode codes.Code
	}{
		{
			name:         "StateETagMismatchReason",
			reason:       StateETagMismatchReason,
			expectedCode: codes.Aborted,
		},
		{
			name:         "StateETagInvalidReason",
			reason:       StateETagInvalidReason,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "TopicNotFoundReason",
			reason:       TopicNotFoundReason,
			expectedCode: codes.NotFound,
		},
		{
			name:         "SecretKeyNotFoundReason",
			reason:       SecretKeyNotFoundReason,
			expectedCode: codes.NotFound,
		},
		{
			name:         "ConfigurationKeyNotFoundReason",
			reason:       ConfigurationKeyNotFoundReason,
			expectedCode: codes.NotFound,
		},
		{
			name:         "NoReason",
			reason:       NoReason,
			expectedCode: codes.Internal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := convertReasonToStatusCode(test.reason)
			assert.Equal(t, test.expectedCode, got, fmt.Sprintf("want %s, got = %s\n", test.expectedCode, got))
		})
	}
}

func TestConvertStatusCodeToHTTPCode(t *testing.T) {
	tests := []struct {
		name             string
		code             codes.Code
		expectedHTTPCode int
	}{
		{
			name:             "Aborted",
			code:             codes.Aborted,
			expectedHTTPCode: 409,
		},
		{
			name:             "InvalidArgument",
			code:             codes.InvalidArgument,
			expectedHTTPCode: 400,
		},
		{
			name:             "NotFound",
			code:             codes.NotFound,
			expectedHTTPCode: 404,
		},
		{
			name:             "Internal",
			code:             codes.Internal,
			expectedHTTPCode: 503,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := convertStatusCodeToHTTPCode(test.code)
			assert.Equal(t, test.expectedHTTPCode, got, fmt.Sprintf("want %d, got = %d\n", test.expectedHTTPCode, got))
		})
	}
}
