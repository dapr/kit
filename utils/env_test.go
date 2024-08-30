package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetEnvIntWithRangeWrongValues(t *testing.T) {
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

			val, err := GetEnvDurationWithRange("MY_ENV", defaultValue, tt.min, tt.max)
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

			val, err := GetEnvDurationWithRange("MY_ENV", 3*time.Second, time.Second, 5*time.Second)
			require.NoError(t, err)
			require.Equal(t, tt.result, val)
		})
	}
}
