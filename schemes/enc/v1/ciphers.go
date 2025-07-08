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

// Cipher used to encrypt the file.
//
//nolint:recvcheck
type Cipher string

const (
	CipherAESGCM           Cipher = "AES-GCM"
	CipherChaCha20Poly1305 Cipher = "CHACHA20-POLY1305"

	cipherInvalid             = 0
	cipherNumAESGCM           = 1
	cipherNumChaCha20Poly1305 = 2
)

// Validate the passed cipher and resolves aliases.
func (c Cipher) Validate() (Cipher, error) {
	switch c {
	// Valid ciphers, not aliased
	case CipherAESGCM, CipherChaCha20Poly1305:
		return c, nil

	default:
		return c, fmt.Errorf("cipher %s is not supported", c)
	}
}

// ID returns the numeric ID for the cipher.
func (c Cipher) ID() int {
	switch c {
	case CipherAESGCM:
		return cipherNumAESGCM
	case CipherChaCha20Poly1305:
		return cipherNumChaCha20Poly1305
	default:
		return cipherInvalid
	}
}

// NewCipherFromID returns a Cipher from its ID.
func NewCipherFromID(id int) (Cipher, error) {
	switch id {
	case cipherNumAESGCM:
		return CipherAESGCM, nil
	case cipherNumChaCha20Poly1305:
		return CipherChaCha20Poly1305, nil
	default:
		return "", fmt.Errorf("cipher ID %d is not supported", id)
	}
}

// MarhsalJSON implements json.Marshaler.
func (c Cipher) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(c.ID())), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Cipher) UnmarshalJSON(dataB []byte) error {
	data := string(dataB)
	if data == "" || data == "null" {
		return errors.New("value is empty")
	}

	id, err := strconv.Atoi(data)
	if err != nil {
		return errors.New("failed to parse value as number")
	}

	newC, err := NewCipherFromID(id)
	if err != nil {
		return err
	}
	*c = newC
	return nil
}
