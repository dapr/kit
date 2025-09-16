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

package config

import (
	"strings"
	"unicode"
)

func PrefixedBy(input any, prefix string) (any, error) {
	normalized, err := Normalize(input)
	if err != nil {
		// The only error that can come from normalize is if
		// input is a map[interface{}]interface{} and contains
		// a key that is not a string.
		return input, err
	}
	input = normalized

	if inputMap, ok := input.(map[string]any); ok {
		converted := make(map[string]any, len(inputMap))
		for k, v := range inputMap {
			if strings.HasPrefix(k, prefix) {
				key := uncapitalize(strings.TrimPrefix(k, prefix))
				converted[key] = v
			}
		}

		return converted, nil
	} else if inputMap, ok := input.(map[string]string); ok {
		converted := make(map[string]string, len(inputMap))
		for k, v := range inputMap {
			if strings.HasPrefix(k, prefix) {
				key := uncapitalize(strings.TrimPrefix(k, prefix))
				converted[key] = v
			}
		}

		return converted, nil
	}

	return input, nil
}

// uncapitalize initial capital letters in `str`.
func uncapitalize(str string) string {
	if len(str) == 0 {
		return str
	}

	vv := []rune(str) // Introduced later
	vv[0] = unicode.ToLower(vv[0])

	return string(vv)
}
