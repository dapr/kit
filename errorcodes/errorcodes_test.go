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
		name                string
		inErr               error
		inErrOptions        []ErrorOption
		inMetadata          map[string]string
		expReason           Reason
		expDescription      string
		expMetadata         map[string]string
		expResourceInfoData *ResourceInfo
		expDe               *DaprError
	}{
		{
			name:  "DaprError_New_OK_No_Reason",
			inErr: &DaprError{},
			inMetadata: map[string]string{
				ErrorCodesFeatureMetadataKey: "true",
			},
			expReason: NoReason,
		},
		{
			name:  "DaprError_New_Nil_Error",
			inErr: nil,
			inMetadata: map[string]string{
				ErrorCodesFeatureMetadataKey: "true",
			},
			expDe: &DaprError{},
		},
		// {
		// 	name:     "DaprError_New_Nil_Metadata",
		// 	inErr:    &DaprError{},
		// 	md:       nil,
		// 	expected: nil,
		// },
		// {
		// 	name:     "DaprError_New_Empty_Metadata",
		// 	inErr:    &DaprError{},
		// 	md:       map[string]string{},
		// 	expected: nil,
		// },
		// {
		// 	name:  "DaprError_New_Details",
		// 	inErr: tde,
		// 	inErrOptions: []ErrorOption{
		// 		WithResourceInfoData(trid),
		// 	},
		// 	md:       tmd,
		// 	expected: tdexp,
		// },
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			NewDaprError(test.inErr, test.inMetadata, test.inErrOptions...)
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
