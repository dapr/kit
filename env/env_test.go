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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetIntWithRangeWrongValues(t *testing.T) {
	testValues := []struct {
		name      string
		envVarVal string
		min       time.Duration
		max       time.Duration
		error     string
	}{
		{
			"should error if value is not a valid time.Duration",
			"0.5",
			time.Second,
			2 * time.Second,
			"invalid time.Duration value 0s for the MY_ENV env variable",
		},
		{
			"should error if value is lower than 1s",
			"0s",
			time.Second,
			10 * time.Second,
			"value should be between 1s and 10s",
		},
		{
			"should error if value is higher than 10s",
			"2m",
			time.Second,
			10 * time.Second,
			"value should be between 1s and 10s",
		},
	}

	defaultValue := 3 * time.Second
	for _, tt := range testValues {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MY_ENV", tt.envVarVal)

			val, err := GetDurationWithRange("MY_ENV", defaultValue, tt.min, tt.max)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.error)
			require.Equal(t, defaultValue, val)
		})
	}
}

func TestGetEnvDurationWithRangeValidValues(t *testing.T) {
	testValues := []struct {
		name      string
		envVarVal string
		result    time.Duration
	}{
		{
			"should return default value if env variable is not set",
			"",
			3 * time.Second,
		},
		{
			"should return result is env variable value is valid",
			"4s",
			4 * time.Second,
		},
	}

	for _, tt := range testValues {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVarVal != "" {
				t.Setenv("MY_ENV", tt.envVarVal)
			}

			val, err := GetDurationWithRange("MY_ENV", 3*time.Second, time.Second, 5*time.Second)
			require.NoError(t, err)
			require.Equal(t, tt.result, val)
		})
	}
}
