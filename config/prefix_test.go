/*
Copyright 2021 The Dapr Authors
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

package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/config"
)

func TestPrefixedBy(t *testing.T) {
	tests := map[string]struct {
		prefix   string
		input    interface{}
		expected interface{}
		err      string
	}{
		"map of string to string": {
			prefix: "test",
			input: map[string]string{
				"":        "",
				"ignore":  "don't include me",
				"testOne": "include me",
				"testTwo": "and me",
			},
			expected: map[string]string{
				"one": "include me",
				"two": "and me",
			},
		},
		"map of string to interface{}": {
			prefix: "test",
			input: map[string]interface{}{
				"":        "",
				"ignore":  "don't include me",
				"testOne": "include me",
				"testTwo": "and me",
			},
			expected: map[string]interface{}{
				"one": "include me",
				"two": "and me",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := config.PrefixedBy(tc.input, tc.prefix)
			if tc.err != "" {
				require.Error(t, err)
				assert.Equal(t, tc.err, err.Error())
			} else {
				assert.Equal(t, tc.expected, actual, "unexpected output")
			}
		})
	}
}
