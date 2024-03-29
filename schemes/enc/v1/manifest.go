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
)

// Manifest contains the properties for the clear-text manifest which is added at the beginning of the encrypted document.
type Manifest struct {
	// Name of the key that can be used to decrypt the message.
	// This is optional, and if specified can be in the format `key` or `key/version`.
	KeyName string `json:"k,omitempty"`
	// ID of the wrapping algorithm used.
	KeyWrappingAlgorithm KeyAlgorithm `json:"kw"`
	// The Wrapped File Key.
	WFK []byte `json:"wfk"`
	// ID of the cipher used.
	Cipher Cipher `json:"cph"`
	// Random sequence of 7 bytes generated by a CSPRNG
	NoncePrefix []byte `json:"np"`
}

// Validate the object and returns no error if everything is fine.
// It also resolves aliases for the key algorithm and cipher.
func (m *Manifest) Validate() (err error) {
	m.KeyWrappingAlgorithm, err = m.KeyWrappingAlgorithm.Validate()
	if err != nil {
		return fmt.Errorf("key wrapping algorithm is invalid: %w", err)
	}
	if len(m.WFK) == 0 {
		return errors.New("wrapped file key is empty")
	}
	m.Cipher, err = m.Cipher.Validate()
	if err != nil {
		return fmt.Errorf("cipher is invalid: %w", err)
	}
	if len(m.NoncePrefix) != NoncePrefixLength {
		return errors.New("nonce prefix is invalid")
	}

	return nil
}
