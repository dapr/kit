/*
Copyright 2022 The Dapr Authors
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

//nolint:nosnakecase
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/dapr/kit/crypto/aescbcaead"
	"github.com/dapr/kit/crypto/aeskw"
	"github.com/dapr/kit/crypto/padding"
)

// SupportedSymmetricAlgorithms returns the list of supported symmetric encryption algorithms.
// This is a subset of the algorithms defined in consts.go.
func SupportedSymmetricAlgorithms() []string {
	return []string{
		Algorithm_A128CBC, Algorithm_A192CBC, Algorithm_A256CBC,
		Algorithm_A128CBC_NOPAD, Algorithm_A192CBC_NOPAD, Algorithm_A256CBC_NOPAD,
		Algorithm_A128GCM, Algorithm_A192GCM, Algorithm_A256GCM,
		Algorithm_A128CBC_HS256, Algorithm_A192CBC_HS384, Algorithm_A256CBC_HS512,
		Algorithm_A128KW, Algorithm_A192KW, Algorithm_A256KW,
		Algorithm_C20P, Algorithm_C20PKW, Algorithm_XC20P, Algorithm_XC20PKW,
	}
}

// EncryptSymmetric encrypts a message using a symmetric key and the specified algorithm.
// Note that "associatedData" is ignored if the cipher does not support labels/AAD.
func EncryptSymmetric(plaintext []byte, algorithm string, key jwk.Key, nonce []byte, associatedData []byte) (ciphertext []byte, tag []byte, err error) {
	var keyBytes []byte
	if key.KeyType() != jwa.OctetSeq || key.Raw(&keyBytes) != nil {
		return nil, nil, ErrKeyTypeMismatch
	}

	switch algorithm {
	case Algorithm_A128CBC, Algorithm_A192CBC, Algorithm_A256CBC,
		Algorithm_A128CBC_NOPAD, Algorithm_A192CBC_NOPAD, Algorithm_A256CBC_NOPAD:
		ciphertext, err = encryptSymmetricAESCBC(plaintext, algorithm, keyBytes, nonce)
		return ciphertext, tag, err

	case Algorithm_A128GCM, Algorithm_A192GCM, Algorithm_A256GCM:
		return encryptSymmetricAESGCM(plaintext, algorithm, keyBytes, nonce, associatedData)

	case Algorithm_A128CBC_HS256, Algorithm_A192CBC_HS384, Algorithm_A256CBC_HS512:
		return encryptSymmetricAESCBCHMAC(plaintext, algorithm, keyBytes, nonce, associatedData)

	case Algorithm_A128KW, Algorithm_A192KW, Algorithm_A256KW:
		ciphertext, err = encryptSymmetricAESKW(plaintext, algorithm, keyBytes)
		return ciphertext, tag, err

	case Algorithm_C20P, Algorithm_C20PKW, Algorithm_XC20P, Algorithm_XC20PKW:
		return encryptSymmetricChaCha20Poly1305(plaintext, algorithm, keyBytes, nonce, associatedData)

	default:
		return nil, nil, ErrUnsupportedAlgorithm
	}
}

// DecryptSymmetric decrypts an encrypted message using a symmetric key and the specified algorithm.
// Note that "associatedData" is ignored if the cipher does not support labels/AAD.
func DecryptSymmetric(ciphertext []byte, algorithm string, key jwk.Key, nonce []byte, tag []byte, associatedData []byte) (plaintext []byte, err error) {
	var keyBytes []byte
	if key.KeyType() != jwa.OctetSeq || key.Raw(&keyBytes) != nil {
		return nil, ErrKeyTypeMismatch
	}

	switch algorithm {
	case Algorithm_A128CBC, Algorithm_A192CBC, Algorithm_A256CBC,
		Algorithm_A128CBC_NOPAD, Algorithm_A192CBC_NOPAD, Algorithm_A256CBC_NOPAD:
		return decryptSymmetricAESCBC(ciphertext, algorithm, keyBytes, nonce)

	case Algorithm_A128GCM, Algorithm_A192GCM, Algorithm_A256GCM:
		return decryptSymmetricAESGCM(ciphertext, algorithm, keyBytes, nonce, tag, associatedData)

	case Algorithm_A128CBC_HS256, Algorithm_A192CBC_HS384, Algorithm_A256CBC_HS512:
		return decryptSymmetricAESCBCHMAC(ciphertext, algorithm, keyBytes, nonce, tag, associatedData)

	case Algorithm_A128KW, Algorithm_A192KW, Algorithm_A256KW:
		return decryptSymmetricAESKW(ciphertext, algorithm, keyBytes)

	case Algorithm_C20P, Algorithm_C20PKW, Algorithm_XC20P, Algorithm_XC20PKW:
		return decryptSymmetricChaCha20Poly1305(ciphertext, algorithm, keyBytes, nonce, tag, associatedData)

	default:
		return nil, ErrUnsupportedAlgorithm
	}
}

func encryptSymmetricAESCBC(plaintext []byte, algorithm string, key []byte, iv []byte) (ciphertext []byte, err error) {
	if len(key) != expectedKeySize(algorithm) {
		return nil, ErrKeyTypeMismatch
	}
	if len(iv) != aes.BlockSize {
		return nil, ErrInvalidNonce
	}

	switch algorithm {
	case Algorithm_A128CBC_NOPAD, Algorithm_A192CBC_NOPAD, Algorithm_A256CBC_NOPAD:
		if (len(plaintext) % aes.BlockSize) != 0 {
			return nil, ErrInvalidPlaintextLength
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrKeyTypeMismatch
	}

	switch algorithm {
	case Algorithm_A128CBC_NOPAD, Algorithm_A192CBC_NOPAD, Algorithm_A256CBC_NOPAD:
		// nop
	default:
		plaintext, err = padding.PadPKCS7(plaintext, aes.BlockSize)
		if err != nil {
			return nil, err
		}
	}

	ciphertext = make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, iv).
		CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

// Note that when using PKCS#7 padding, this returns a specific error if padding mismatches.
// Callers are responsible for handling these errors in a way that doesn't introduce the possibility of padding oracle attacks.
// See: https://research.nccgroup.com/2021/02/17/cryptopals-exploiting-cbc-padding-oracles/
func decryptSymmetricAESCBC(ciphertext []byte, algorithm string, key []byte, iv []byte) (plaintext []byte, err error) {
	if len(key) != expectedKeySize(algorithm) {
		return nil, ErrKeyTypeMismatch
	}
	if len(iv) != aes.BlockSize {
		return nil, ErrInvalidNonce
	}
	if (len(ciphertext) % aes.BlockSize) != 0 {
		return nil, ErrInvalidCiphertextLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrKeyTypeMismatch
	}

	plaintext = make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).
		CryptBlocks(plaintext, ciphertext)

	switch algorithm {
	case Algorithm_A128CBC_NOPAD, Algorithm_A192CBC_NOPAD, Algorithm_A256CBC_NOPAD:
		// nop
	default:
		plaintext, err = padding.UnpadPKCS7(plaintext, aes.BlockSize)
		if err != nil {
			return nil, err
		}
	}

	return plaintext, nil
}

func encryptSymmetricAESGCM(plaintext []byte, algorithm string, key []byte, nonce []byte, associatedData []byte) (ciphertext []byte, tag []byte, err error) {
	if len(key) != expectedKeySize(algorithm) {
		return nil, nil, ErrKeyTypeMismatch
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, ErrKeyTypeMismatch
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, ErrKeyTypeMismatch
	}

	return encryptSymmetricAEAD(aead, plaintext, algorithm, key, nonce, associatedData)
}

func encryptSymmetricAESCBCHMAC(plaintext []byte, algorithm string, key []byte, nonce []byte, associatedData []byte) (ciphertext []byte, tag []byte, err error) {
	aead, err := getAESCBCHMACCipher(algorithm, key)
	if err != nil {
		return nil, nil, err
	}

	return encryptSymmetricAEAD(aead, plaintext, algorithm, key, nonce, associatedData)
}

func encryptSymmetricAEAD(aead cipher.AEAD, plaintext []byte, algorithm string, key []byte, nonce []byte, associatedData []byte) (ciphertext []byte, tag []byte, err error) {
	if len(nonce) != aead.NonceSize() {
		return nil, nil, ErrInvalidNonce
	}

	out := aead.Seal(nil, nonce, plaintext, associatedData)
	// Tag is added at the end
	tagSize := aead.Overhead()
	return out[0 : len(out)-tagSize], out[len(out)-tagSize:], nil
}

func decryptSymmetricAESGCM(ciphertext []byte, algorithm string, key []byte, nonce []byte, tag []byte, associatedData []byte) (plaintext []byte, err error) {
	if len(key) != expectedKeySize(algorithm) {
		return nil, ErrKeyTypeMismatch
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrKeyTypeMismatch
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrKeyTypeMismatch
	}

	return decryptSymmetricAEAD(aead, ciphertext, algorithm, key, nonce, tag, associatedData)
}

func decryptSymmetricAESCBCHMAC(ciphertext []byte, algorithm string, key []byte, nonce []byte, tag []byte, associatedData []byte) (plaintext []byte, err error) {
	aead, err := getAESCBCHMACCipher(algorithm, key)
	if err != nil {
		return nil, err
	}

	return decryptSymmetricAEAD(aead, ciphertext, algorithm, key, nonce, tag, associatedData)
}

func decryptSymmetricAEAD(aead cipher.AEAD, ciphertext []byte, algorithm string, key []byte, nonce []byte, tag []byte, associatedData []byte) (plaintext []byte, err error) {
	if len(nonce) != aead.NonceSize() {
		return nil, ErrInvalidNonce
	}

	if len(tag) != aead.Overhead() {
		return nil, ErrInvalidTag
	}

	// Add the tag at the end of the ciphertext
	ciphertext = append(ciphertext, tag...)
	return aead.Open(nil, nonce, ciphertext, associatedData)
}

func encryptSymmetricAESKW(plaintext []byte, algorithm string, key []byte) (ciphertext []byte, err error) {
	if len(key) != expectedKeySize(algorithm) {
		return nil, ErrKeyTypeMismatch
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrKeyTypeMismatch
	}

	return aeskw.Wrap(block, plaintext)
}

func decryptSymmetricAESKW(ciphertext []byte, algorithm string, key []byte) (plaintext []byte, err error) {
	if len(key) != expectedKeySize(algorithm) {
		return nil, ErrKeyTypeMismatch
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrKeyTypeMismatch
	}

	return aeskw.Unwrap(block, ciphertext)
}

func encryptSymmetricChaCha20Poly1305(plaintext []byte, algorithm string, key []byte, nonce []byte, associatedData []byte) (ciphertext []byte, tag []byte, err error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, nil, ErrKeyTypeMismatch
	}

	aead, err := getChaCha20Poly1305Cipher(algorithm, key, nonce)
	if err != nil {
		return nil, nil, err
	}

	// Tag is added at the end
	out := aead.Seal(nil, nonce, plaintext, associatedData)
	return out[0 : len(out)-chacha20poly1305.Overhead], out[len(out)-chacha20poly1305.Overhead:], nil
}

func decryptSymmetricChaCha20Poly1305(ciphertext []byte, algorithm string, key []byte, nonce []byte, tag []byte, associatedData []byte) (plaintext []byte, err error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, ErrKeyTypeMismatch
	}

	aead, err := getChaCha20Poly1305Cipher(algorithm, key, nonce)
	if err != nil {
		return nil, err
	}

	if len(tag) != aead.Overhead() {
		return nil, ErrInvalidTag
	}

	// Add the tag at the end of the ciphertext
	ciphertext = append(ciphertext, tag...)
	return aead.Open(nil, nonce, ciphertext, associatedData)
}

func getChaCha20Poly1305Cipher(algorithm string, key []byte, nonce []byte) (aead cipher.AEAD, err error) {
	switch algorithm {
	case Algorithm_C20P, Algorithm_C20PKW:
		aead, err = chacha20poly1305.New(key)
		if err == nil && len(nonce) != chacha20poly1305.NonceSize {
			err = ErrInvalidNonce
		}
		return

	case Algorithm_XC20P, Algorithm_XC20PKW:
		aead, err = chacha20poly1305.NewX(key)
		if err == nil && len(nonce) != chacha20poly1305.NonceSizeX {
			err = ErrInvalidNonce
		}
		return
	}

	return nil, errors.New("invalid algorithm")
}

func getAESCBCHMACCipher(algorithm string, key []byte) (aead cipher.AEAD, err error) {
	switch algorithm {
	case Algorithm_A128CBC_HS256:
		if len(key) != 32 {
			return nil, ErrKeyTypeMismatch
		}
		aead, err = aescbcaead.NewAESCBC128SHA256(key)
	case Algorithm_A192CBC_HS384:
		if len(key) != 48 {
			return nil, ErrKeyTypeMismatch
		}
		aead, err = aescbcaead.NewAESCBC192SHA384(key)
	case Algorithm_A256CBC_HS512:
		if len(key) != 64 {
			return nil, ErrKeyTypeMismatch
		}
		aead, err = aescbcaead.NewAESCBC256SHA512(key)
	default:
		return nil, errors.New("invalid algorithm")
	}
	if err != nil {
		return nil, ErrKeyTypeMismatch
	}
	return aead, nil
}

func expectedKeySize(alg string) int {
	switch alg[1:4] {
	case "128":
		return 16
	case "192":
		return 24
	case "256":
		return 32
	}
	return 0
}
