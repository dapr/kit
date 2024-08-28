package utils

import (
	"fmt"
	"os"
	"strconv"
)

// GetEnvIntWithRange returns the integer value of the environment variable specified by `envVar`.
// If the environment variable is not set, it returns `defaultValue`.
// If the value is set but is not valid (not a valid integer or falls outside the specified range
// [minValue, maxValue]), it returns `defaultValue` and an error.
func GetEnvIntWithRange(envVar string, defaultValue int, min int, max int) (int, error) {
	v := os.Getenv(envVar)
	if v == "" {
		return defaultValue, nil
	}

	val, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue, fmt.Errorf("invalid integer value for the %s env variable: %w", envVar, err)
	}

	if val < min || val > max {
		return defaultValue, fmt.Errorf("invalid value for the %s env variable: value should be between %d and %d for best performance, got %d", envVar, min, max, val)
	}

	return val, nil
}
