/*
Copyright 2024 The Dapr Authors
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

package env

import (
	"fmt"
	"os"
	"time"
)

// GetDurationWithRange returns the time.Duration value of the environment variable specified by `envVar`.
// If the environment variable is not set, it returns `defaultValue`.
// If the value is set but is not valid (not a valid time.Duration or falls outside the specified range
// [minValue, maxValue] inclusively), it returns `defaultValue` and an error.
func GetDurationWithRange(envVar string, defaultValue, min, max time.Duration) (time.Duration, error) {
	v := os.Getenv(envVar)
	if v == "" {
		return defaultValue, nil
	}

	val, err := time.ParseDuration(v)
	if err != nil {
		return defaultValue, fmt.Errorf("invalid time.Duration value %s for the %s env variable: %w", val, envVar, err)
	}

	if val < min || val > max {
		return defaultValue, fmt.Errorf("invalid value for the %s env variable: value should be between %s and %s, got %s", envVar, min, max, val)
	}

	return val, nil
}
