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

func TestNormalize(t *testing.T) {
	tests := map[string]struct {
		input    any
		expected any
		err      string
	}{
		"simple": {input: "test", expected: "test"},
		"map of string to interface{}": {
			input: map[string]any{
				"test": "1234",
				"nested": map[string]any{
					"value": "5678",
				},
			}, expected: map[string]any{
				"test": "1234",
				"nested": map[string]any{
					"value": "5678",
				},
			},
		},
		"map of string to interface{} with error": {
			input: map[string]any{
				"test": "1234",
				"nested": map[any]any{
					5678: "5678",
				},
			}, err: "error parsing config field: 5678",
		},
		"map of interface{} to interface{}": {
			input: map[string]any{
				"test": "1234",
				"nested": map[any]any{
					"value": "5678",
				},
			}, expected: map[string]any{
				"test": "1234",
				"nested": map[string]any{
					"value": "5678",
				},
			},
		},
		"map of interface{} to interface{} with error": {
			input: map[any]any{
				"test": "1234",
				"nested": map[any]any{
					5678: "5678",
				},
			}, err: "error parsing config field: 5678",
		},
		"slice of interface{}": {
			input: []any{
				map[any]any{
					"value": "5678",
				},
			}, expected: []any{
				map[string]any{
					"value": "5678",
				},
			},
		},
		"slice of interface{} with error": {
			input: []any{
				map[any]any{
					1234: "1234",
				},
			}, err: "error parsing config field: 1234",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := config.Normalize(tc.input)
			if tc.err != "" {
				require.Error(t, err)
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expected, actual)
		})
	}
}
