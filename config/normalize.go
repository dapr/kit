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
	"fmt"
)

// Normalize converts map[interface{}]interface{} to map[string]interface{} to normalize
// for JSON and usage in component initialization.
//
//nolint:cyclop
func Normalize(i interface{}) (interface{}, error) {
	var err error
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			if strKey, ok := k.(string); ok {
				if m2[strKey], err = Normalize(v); err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("error parsing config field: %v", k)
			}
		}

		return m2, nil
	case map[string]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			if m2[k], err = Normalize(v); err != nil {
				return nil, err
			}
		}

		return m2, nil
	case []interface{}:
		for i, v := range x {
			if x[i], err = Normalize(v); err != nil {
				return nil, err
			}
		}
	}

	return i, nil
}
