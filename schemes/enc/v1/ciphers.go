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
	"strconv"
)

// Cipher used to encrypt the file.
type Cipher string

const (
	CipherAESGCM           Cipher = "AES-GCM"
	CipherChaCha20Poly1305 Cipher = "CHACHA20-POLY1305"
)

// Validate the passed cipher and resolves aliases.
func (a Cipher) Validate() (Cipher, error) {
	switch a {
	// Valid ciphers, not aliased
	case CipherAESGCM, CipherChaCha20Poly1305:
		return a, nil

	default:
		return a, errors.New("cipher " + string(a) + " is not supported")
	}
}

// ID returns the numeric ID for the cipher.
func (a Cipher) ID() int {
	switch a {
	case CipherAESGCM:
		return 1
	case CipherChaCha20Poly1305:
		return 2
	default:
		return 0
	}
}

// NewCipherFromID returns a Cipher from its ID.
func NewCipherFromID(id int) (Cipher, error) {
	switch id {
	case 1:
		return CipherAESGCM, nil
	case 2:
		return CipherChaCha20Poly1305, nil
	default:
		return "", errors.New("cipher ID " + strconv.Itoa(id) + " is not supported")
	}
}

// MarhsalJSON implements json.Marshaler.
func (a Cipher) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(a.ID())), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *Cipher) UnmarshalJSON(dataB []byte) error {
	data := string(dataB)
	if data == "" || data == "null" {
		return errors.New("value is empty")
	}

	id, err := strconv.Atoi(data)
	if err != nil {
		return errors.New("failed to parse value as number")
	}

	newA, err := NewCipherFromID(id)
	if err != nil {
		return err
	}
	*a = newA
	return nil
}
