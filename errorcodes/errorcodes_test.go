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
	md := map[string]string{}
	ActivateErrorCodesFeature(true, md)
	if _, ok := md[ErrorCodesFeatureMetadataKey]; !ok {
		t.Errorf("expected metadata to contain the value of 'error_codes_feature' ")
	}

	md = map[string]string{}
	ActivateErrorCodesFeature(false, md)
	if _, ok := md[ErrorCodesFeatureMetadataKey]; ok {
		t.Errorf("expected metadata NOT to contain the value of 'error_codes_feature' ")
	}
}

func TestNew(t *testing.T) {
	if de := New(nil, nil); de == nil {
		assert.Nil(t, de)
	}
}

func TestFromDaprErrorToGRPC(t *testing.T) {
	if _, e := FromDaprErrorToGRPC(fmt.Errorf("bad error")); e == nil {
		t.Errorf("expected failure")
	}
}

func TestFromDaprErrorToHTTP(t *testing.T) {
	if _, _, e := FromDaprErrorToHTTP(fmt.Errorf("bad error")); e == nil {
		t.Errorf("expected failure")
	}
}
