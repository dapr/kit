/*
Copyright 2025 The Dapr Authors
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

package slices

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Deduplicate(t *testing.T) {
	tests := []struct {
		input []int
		exp   []int
	}{
		{
			input: []int{1, 2, 3},
			exp:   []int{1, 2, 3},
		},
		{
			input: []int{1, 2, 2, 3, 1},
			exp:   []int{1, 2, 3},
		},
		{
			input: []int{5, 5, 5, 5},
			exp:   []int{5},
		},
		{
			input: []int{},
			exp:   []int{},
		},
		{
			input: []int{42},
			exp:   []int{42},
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.input), func(t *testing.T) {
			assert.ElementsMatch(t, test.exp, Deduplicate(test.input))
		})
	}
}
