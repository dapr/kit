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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// fileKey holds the fileKey and uses that (and the haeaderKey and payloadKey it derives from it)
// to perform the actual cryptographic operations in the package.
// This object is also used encrypt/decrypt each segment, and to compute the MAC of the header.
type fileKey struct {
	cipher Cipher

	fileKey     []byte
	noncePrefix []byte

	// HMAC key used to sign the header
	headerKey []byte
	// Key used to encrypt the payload
	payloadKey []byte
}

func newFileKey(cipher Cipher) (fileKey, error) {
	// Read 39 random bytes for the file key (256 bits) and nonce prefix (56 bits)
	rnd := make([]byte, 39)
	_, err := io.ReadFull(rand.Reader, rnd)
	if err != nil {
		return fileKey{}, fmt.Errorf("failed to generate file key: %w", err)
	}

	// Return the object
	return importFileKey(rnd[0:32], rnd[32:39], cipher)
}

func importFileKey(fileKey, noncePrefix []byte, cipher Cipher) (fk fileKey, err error) {
	// Set the properties in the object
	fk.fileKey = fileKey
	fk.noncePrefix = noncePrefix
	fk.cipher = cipher

	// Derive the keys
	fk.headerKey, err = fk.deriveKey(32, []byte("header"), nil)
	if err != nil {
		return fk, fmt.Errorf("failed to derive the header key: %w", err)
	}
	fk.payloadKey, err = fk.deriveKey(32, []byte("payload"), fk.noncePrefix)
	if err != nil {
		return fk, fmt.Errorf("failed to derive the payload key: %w", err)
	}

	return fk, nil
}

// Returns the file key.
func (k fileKey) GetFileKey() []byte {
	return k.fileKey
}

// Returns the nonce prefix.
func (k fileKey) GetNoncePrefix() []byte {
	return k.noncePrefix
}

// Returns the signed header given a manifest.
func (k fileKey) SignHeader(manifest []byte) ([]byte, error) {
	// Message to sign
	msg := k.headerMessage(manifest)

	// Compute the MAC
	mac, err := k.computeHeaderSignature(msg)
	if err != nil {
		return nil, err
	}

	// Create the output
	// This contains the plain-text header (already ending with a newline), the base64-encoded MAC, and a final newline
	res := make([]byte, len(msg)+base64.StdEncoding.EncodedLen(len(mac))+1)
	copy(res, msg)
	base64.StdEncoding.Encode(res[len(msg):], mac)
	res[len(res)-1] = '\n'

	// The header must not be bigger than 64KB
	if len(res) > SegmentSize {
		return nil, errors.New("header is too long")
	}

	return res, nil
}

// Verifies the signature of the header given a manifest and the base64-encoded MAC
func (k fileKey) VerifyHeaderSignature(manifest []byte, macB64 []byte) error {
	// Decode the base64-encoded MAC
	mac := make([]byte, base64.StdEncoding.DecodedLen(len(macB64)))
	n, err := base64.StdEncoding.Decode(mac, macB64)
	if err != nil {
		return fmt.Errorf("failed to decode header's signature: %w", err)
	}
	mac = mac[:n]

	// Message to sign
	msg := k.headerMessage(manifest)

	// Compute the expected MAC
	expectMAC, err := k.computeHeaderSignature(msg)
	if err != nil {
		return err
	}

	// Check the MAC using constant time comparison
	if subtle.ConstantTimeCompare(expectMAC, mac) != 1 {
		return ErrDecryptionSignature
	}

	// All good
	return nil
}

// Returns the header's message (which will be signed)
func (k fileKey) headerMessage(manifest []byte) []byte {
	return bytes.Join([][]byte{
		[]byte(SchemeName),
		manifest,
		{}, // End with a newline
	}, []byte{'\n'})
}

// Compute the signature of the header
func (k fileKey) computeHeaderSignature(msg []byte) ([]byte, error) {
	h := hmac.New(sha256.New, k.headerKey)
	_, err := h.Write(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to write into HMAC: %w", err)
	}
	mac := h.Sum(nil)
	return mac, nil
}

// Type for both EncryptSegment and DecryptSegment.
type processSegmentFn = func(out io.Writer, data []byte, num uint32, last bool) error

// Encrypt a segment of data and write it into the writable stream.
func (k fileKey) EncryptSegment(out io.Writer, data []byte, num uint32, last bool) error {
	l := len(data)
	if l == 0 {
		return errors.New("input plaintext is empty")
	}

	// Get the cipher
	aead, err := k.getCipher()
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create the nonce for the segment
	nonce := k.nonceForSegment(num, last)

	// Encrypt the segment, re-using the same buffer for the output
	data = aead.Seal(data[:0], nonce, data, nil)

	// Write the output to the destination stream
	_, err = out.Write(data[0:(l + aead.Overhead())])
	if err != nil {
		return fmt.Errorf("error writing encrypted segment to output stream: %w", err)
	}
	return nil
}

// Decrypt a segment of data it write it into the writable stream.
func (k fileKey) DecryptSegment(out io.Writer, data []byte, num uint32, last bool) error {
	l := len(data)
	if l == 0 {
		return errors.New("input ciphertext is empty")
	}

	// Get the cipher
	aead, err := k.getCipher()
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create the nonce for the segment
	nonce := k.nonceForSegment(num, last)

	// Decrypt the segment, re-using the same buffer for the output
	data, err = aead.Open(data[:0], nonce, data, nil)
	if err != nil {
		return ErrDecryptionFailed
	}

	// Write the output to the destination stream
	_, err = out.Write(data[0:(l - aead.Overhead())])
	if err != nil {
		return fmt.Errorf("error writing decrypted segment to output stream: %w", err)
	}
	return nil
}

// Computes the nonce for a segment.
func (k fileKey) nonceForSegment(num uint32, last bool) []byte {
	nonce := make([]byte, 12)
	copy(nonce[0:7], k.noncePrefix)
	binary.BigEndian.PutUint32(nonce[7:11], num)
	if last {
		nonce[11] = 0x1
	} else {
		nonce[11] = 0x0
	}
	return nonce
}

// Returns the cipher object.
func (k fileKey) getCipher() (aead cipher.AEAD, err error) {
	switch k.cipher {
	case CipherAESGCM:
		var block cipher.Block
		block, err = aes.NewCipher(k.payloadKey)
		if err != nil {
			return nil, err
		}
		aead, err = cipher.NewGCM(block)

	case CipherChaCha20Poly1305:
		aead, err = chacha20poly1305.New(k.payloadKey)

	default:
		err = errors.New("unsupported cipher: " + string(k.cipher))
	}

	return aead, err
}

// Derives a key from the file key using HKDF-SHA-256.
// This is used for both the headerKey and payloadKey.
func (k fileKey) deriveKey(size int, info []byte, salt []byte) ([]byte, error) {
	hkdf := hkdf.New(sha256.New, k.fileKey, salt, info)
	key := make([]byte, size)
	_, err := io.ReadFull(hkdf, key)
	if err != nil {
		return nil, fmt.Errorf("error from HKDF function: %w", err)
	}
	return key, nil
}
