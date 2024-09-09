package utils

import (
	"fmt"
	"os"
	"time"
)

// GetEnvDurationWithRange returns the time.Duration value of the environment variable specified by `envVar`.
// If the environment variable is not set, it returns `defaultValue`.
// If the value is set but is not valid (not a valid time.Duration or falls outside the specified range
// [minValue, maxValue] inclusively), it returns `defaultValue` and an error.
func GetEnvDurationWithRange(envVar string, defaultValue, min, max time.Duration) (time.Duration, error) {
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
