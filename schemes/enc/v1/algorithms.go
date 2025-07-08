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

package v1

import (
	"errors"
	"fmt"
	"strconv"
)

// Algorithm used to wrap the file key.
//
//nolint:recvcheck
type KeyAlgorithm string

const (
	KeyAlgorithmAES256KW   KeyAlgorithm = "A256KW"
	KeyAlgorithmAES128CBC  KeyAlgorithm = "A128CBC-NOPAD"
	KeyAlgorithmAES192CBC  KeyAlgorithm = "A192CBC-NOPAD"
	KeyAlgorithmAES256CBC  KeyAlgorithm = "A256CBC-NOPAD"
	KeyAlgorithmRSAOAEP256 KeyAlgorithm = "RSA-OAEP-256"

	KeyAlgorithmAES KeyAlgorithm = "AES" // Alias for A256KW
	KeyAlgorithmRSA KeyAlgorithm = "RSA" // Alias for RSA-OAEP-256

	keyAlgorithmInvalid       = 0
	keyAlgorithmNumAES256KW   = 1
	keyAlgorithmNumAES128CBC  = 2
	keyAlgorithmNumAES192CBC  = 3
	keyAlgorithmNumAES256CBC  = 4
	keyAlgorithmNumRSAOAEP256 = 5
)

// Validate the passed algorithm and resolves aliases.
func (a KeyAlgorithm) Validate() (KeyAlgorithm, error) {
	switch a {
	// Valid algorithms, not aliased
	case KeyAlgorithmAES256KW,
		KeyAlgorithmAES128CBC, KeyAlgorithmAES192CBC, KeyAlgorithmAES256CBC,
		KeyAlgorithmRSAOAEP256:
		return a, nil

	// Alias for A256KW
	case KeyAlgorithmAES:
		return KeyAlgorithmAES256KW, nil

	// Alias for RSA-OAEP-256
	case KeyAlgorithmRSA:
		return KeyAlgorithmRSAOAEP256, nil

	default:
		return a, fmt.Errorf("algorithm %s is not supported", a)
	}
}

// ID returns the numeric ID for the algorithm.
func (a KeyAlgorithm) ID() int {
	switch a {
	case KeyAlgorithmAES256KW, KeyAlgorithmAES:
		return keyAlgorithmNumAES256KW
	case KeyAlgorithmAES128CBC:
		return keyAlgorithmNumAES128CBC
	case KeyAlgorithmAES192CBC:
		return keyAlgorithmNumAES192CBC
	case KeyAlgorithmAES256CBC:
		return keyAlgorithmNumAES256CBC
	case KeyAlgorithmRSAOAEP256, KeyAlgorithmRSA:
		return keyAlgorithmNumRSAOAEP256
	default:
		return keyAlgorithmInvalid
	}
}

// NewKeyAlgorithmFromID returns a KeyAlgorithm from its ID.
func NewKeyAlgorithmFromID(id int) (KeyAlgorithm, error) {
	switch id {
	case keyAlgorithmNumAES256KW:
		return KeyAlgorithmAES256KW, nil
	case keyAlgorithmNumAES128CBC:
		return KeyAlgorithmAES128CBC, nil
	case keyAlgorithmNumAES192CBC:
		return KeyAlgorithmAES192CBC, nil
	case keyAlgorithmNumAES256CBC:
		return KeyAlgorithmAES256CBC, nil
	case keyAlgorithmNumRSAOAEP256:
		return KeyAlgorithmRSAOAEP256, nil
	default:
		return "", fmt.Errorf("algorithm ID %d is not supported", id)
	}
}

// MarhsalJSON implements json.Marshaler.
func (a KeyAlgorithm) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(a.ID())), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *KeyAlgorithm) UnmarshalJSON(dataB []byte) error {
	data := string(dataB)
	if data == "" || data == "null" {
		return errors.New("value is empty")
	}

	id, err := strconv.Atoi(data)
	if err != nil {
		return errors.New("failed to parse value as number")
	}

	newA, err := NewKeyAlgorithmFromID(id)
	if err != nil {
		return err
	}
	*a = newA
	return nil
}
