package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEnvIntWithRangeWrongValues(t *testing.T) {
	defaultValue := 3

	testValues := []struct {
		name      string
		envVarVal string
		min       int
		max       int
		error     string
	}{
		{
			"should error if value is not integer number",
			"0.5",
			1,
			2,
			"invalid integer value for the MY_ENV env variable",
		},
		{
			"should error if value is not integer",
			"abc",
			1,
			2,
			"invalid integer value for the MY_ENV env variable",
		},
		{
			"should error if value is lower than 1",
			"0",
			1,
			10,
			"value should be between 1 and 10",
		},
		{
			"should error if value is higher than 10",
			"11",
			1,
			10,
			"value should be between 1 and 10",
		},
	}

	for _, tt := range testValues {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MY_ENV", tt.envVarVal)

			val, err := GetEnvIntWithRange("MY_ENV", defaultValue, tt.min, tt.max)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.error)
			require.Equal(t, defaultValue, val)
		})
	}
}

func TestGetEnvIntWithRangeValidValues(t *testing.T) {
	testValues := []struct {
		name      string
		envVarVal string
		result    int
	}{
		{
			"should return default value if env variable is not set",
			"",
			3,
		},
		{
			"should return result is env variable value is valid",
			"4",
			4,
		},
	}

	for _, tt := range testValues {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVarVal != "" {
				t.Setenv("MY_ENV", tt.envVarVal)
			}

			val, err := GetEnvIntWithRange("MY_ENV", 3, 1, 5)
			require.NoError(t, err)
			require.Equal(t, tt.result, val)
		})
	}
}
