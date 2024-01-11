/*
Copyright 2023 The Dapr Authors
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

package utils

import (
	"encoding/pem"
	"fmt"
	"os"
)

// GetPEM loads a PEM-encoded file (certificate or key).
func GetPEM(val string) ([]byte, error) {
	// If val is already a PEM-encoded string, return it as-is
	if IsValidPEM(val) {
		return []byte(val), nil
	}

	// Assume it's a file
	pemBytes, err := os.ReadFile(val)
	if err != nil {
		return nil, fmt.Errorf("value is neither a valid file path or nor a valid PEM-encoded string: %w", err)
	}
	return pemBytes, nil
}

// IsValidPEM validates the provided input has PEM formatted block.
func IsValidPEM(val string) bool {
	block, _ := pem.Decode([]byte(val))
	return block != nil
}
