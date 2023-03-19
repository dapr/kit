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
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/crypto/padding"
)

func TestEncryptSymmetricAESCBC(t *testing.T) {
	type args struct {
		plaintext []byte
		algorithm string
		key       []byte
		iv        []byte
	}
	type test struct {
		name           string
		args           args
		wantCiphertext []byte
		wantErr        error
	}
	tests := []test{
		{
			name: "key size mismatch",
			args: args{
				algorithm: Algorithm_A128CBC,
				key:       []byte{0x00, 0x01},
				iv:        mustDecodeHexString("000102030405060708090a0b0c0d0e0f"),
				plaintext: mustDecodeHexString("6bc1bee22e409f96e93d7e117393172a"),
			},
			wantErr: ErrKeyTypeMismatch,
		},
		{
			name: "iv size mismatch",
			args: args{
				algorithm: Algorithm_A128CBC,
				key:       mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				iv:        []byte{0x00, 0x01},
				plaintext: mustDecodeHexString("6bc1bee22e409f96e93d7e117393172a"),
			},
			wantErr: ErrInvalidNonce,
		},
	}

	// Test vectors from NIST publication SP800-38A with added padding
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-cbc") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm: v.Algorithm,
				key:       mustDecodeHexString(v.Key),
				iv:        mustDecodeHexString(v.Nonce),
				plaintext: mustDecodeHexString(v.Plaintext),
			},
			wantCiphertext: mustDecodeHexString(v.Ciphertext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCiphertext, err := encryptSymmetricAESCBC(tt.args.plaintext, tt.args.algorithm, tt.args.key, tt.args.iv)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("encryptSymmetricAESCBC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCiphertext, tt.wantCiphertext) {
				t.Errorf("encryptSymmetricAESCBC() = %v, want %v", gotCiphertext, tt.wantCiphertext)
			}
		})
	}
}

func TestEncryptSymmetricAESCBCNOPAD(t *testing.T) {
	type args struct {
		plaintext []byte
		algorithm string
		key       []byte
		iv        []byte
	}
	type test struct {
		name           string
		args           args
		wantCiphertext []byte
		wantErr        error
	}
	tests := []test{
		{
			name: "key size mismatch",
			args: args{
				algorithm: Algorithm_A128CBC_NOPAD,
				key:       []byte{0x00, 0x01},
				iv:        mustDecodeHexString("000102030405060708090a0b0c0d0e0f"),
				plaintext: mustDecodeHexString("6bc1bee22e409f96e93d7e117393172a"),
			},
			wantErr: ErrKeyTypeMismatch,
		},
		{
			name: "iv size mismatch",
			args: args{
				algorithm: Algorithm_A128CBC_NOPAD,
				key:       mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				iv:        []byte{0x00, 0x01},
				plaintext: mustDecodeHexString("6bc1bee22e409f96e93d7e117393172a"),
			},
			wantErr: ErrInvalidNonce,
		},
		{
			name: "invalid plaintext length",
			args: args{
				algorithm: Algorithm_A128CBC_NOPAD,
				key:       mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				iv:        mustDecodeHexString("000102030405060708090a0b0c0d0e0f"),
				plaintext: mustDecodeHexString("0011"),
			},
			wantErr: ErrInvalidPlaintextLength,
		},
	}

	// Test vectors from NIST publication SP800-38A
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-cbc-nopad") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm: v.Algorithm,
				key:       mustDecodeHexString(v.Key),
				iv:        mustDecodeHexString(v.Nonce),
				plaintext: mustDecodeHexString(v.Plaintext),
			},
			wantCiphertext: mustDecodeHexString(v.Ciphertext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCiphertext, err := encryptSymmetricAESCBC(tt.args.plaintext, tt.args.algorithm, tt.args.key, tt.args.iv)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("encryptSymmetricAESCBC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCiphertext, tt.wantCiphertext) {
				t.Errorf("encryptSymmetricAESCBC() = %v, want %v", gotCiphertext, tt.wantCiphertext)
			}
		})
	}
}

func TestDecryptSymmetricAESCBC(t *testing.T) {
	type args struct {
		ciphertext []byte
		algorithm  string
		key        []byte
		iv         []byte
	}
	type test struct {
		name          string
		args          args
		wantPlaintext []byte
		wantErr       error
	}
	tests := []test{
		{
			name: "key size mismatch",
			args: args{
				algorithm:  Algorithm_A128CBC,
				key:        []byte{0x00, 0x01},
				iv:         mustDecodeHexString("000102030405060708090a0b0c0d0e0f"),
				ciphertext: mustDecodeHexString("00000000000000000000000000000000"),
			},
			wantErr: ErrKeyTypeMismatch,
		},
		{
			name: "iv size mismatch",
			args: args{
				algorithm:  Algorithm_A128CBC,
				key:        mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				iv:         []byte{0x00, 0x01},
				ciphertext: mustDecodeHexString("00000000000000000000000000000000"),
			},
			wantErr: ErrInvalidNonce,
		},
		{
			name: "invalid padding",
			args: args{
				algorithm:  Algorithm_A128CBC,
				key:        mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				iv:         mustDecodeHexString("5086cb9b507219ee95db113a917678b2"),
				ciphertext: mustDecodeHexString("73bed6b8e3c1743b7116e69e22229516f6eccda327bf8e5ec43718b0039adcea"),
			},
			wantErr: padding.ErrInvalidPKCS7Padding,
		},
	}

	// Test vectors from NIST publication SP800-38A
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-cbc") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm:  v.Algorithm,
				key:        mustDecodeHexString(v.Key),
				iv:         mustDecodeHexString(v.Nonce),
				ciphertext: mustDecodeHexString(v.Ciphertext),
			},
			wantPlaintext: mustDecodeHexString(v.Plaintext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPlaintext, err := decryptSymmetricAESCBC(tt.args.ciphertext, tt.args.algorithm, tt.args.key, tt.args.iv)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("decryptSymmetricAESCBC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPlaintext, tt.wantPlaintext) {
				t.Errorf("decryptSymmetricAESCBC() = %v, want %v", gotPlaintext, tt.wantPlaintext)
			}
		})
	}
}

func TestDecryptSymmetricAESCBCNOPAD(t *testing.T) {
	type args struct {
		ciphertext []byte
		algorithm  string
		key        []byte
		iv         []byte
	}
	type test struct {
		name          string
		args          args
		wantPlaintext []byte
		wantErr       error
	}
	tests := []test{
		{
			name: "key size mismatch",
			args: args{
				algorithm:  Algorithm_A128CBC_NOPAD,
				key:        []byte{0x00, 0x01},
				iv:         mustDecodeHexString("000102030405060708090a0b0c0d0e0f"),
				ciphertext: mustDecodeHexString("00000000000000000000000000000000"),
			},
			wantErr: ErrKeyTypeMismatch,
		},
		{
			name: "iv size mismatch",
			args: args{
				algorithm:  Algorithm_A128CBC_NOPAD,
				key:        mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				iv:         []byte{0x00, 0x01},
				ciphertext: mustDecodeHexString("00000000000000000000000000000000"),
			},
			wantErr: ErrInvalidNonce,
		},
		{
			name: "invalid ciphertext length",
			args: args{
				algorithm:  Algorithm_A128CBC_NOPAD,
				key:        mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				iv:         mustDecodeHexString("000102030405060708090a0b0c0d0e0f"),
				ciphertext: mustDecodeHexString("0011"),
			},
			wantErr: ErrInvalidCiphertextLength,
		},
	}

	// Test vectors from NIST publication SP800-38A
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-cbc-nopad") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm:  v.Algorithm,
				key:        mustDecodeHexString(v.Key),
				iv:         mustDecodeHexString(v.Nonce),
				ciphertext: mustDecodeHexString(v.Ciphertext),
			},
			wantPlaintext: mustDecodeHexString(v.Plaintext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPlaintext, err := decryptSymmetricAESCBC(tt.args.ciphertext, tt.args.algorithm, tt.args.key, tt.args.iv)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("decryptSymmetricAESCBC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPlaintext, tt.wantPlaintext) {
				t.Errorf("decryptSymmetricAESCBC() = %v, want %v", gotPlaintext, tt.wantPlaintext)
			}
		})
	}
}

func TestEncryptSymmetricAESGCM(t *testing.T) {
	type args struct {
		plaintext      []byte
		algorithm      string
		key            []byte
		nonce          []byte
		associatedData []byte
	}
	type test struct {
		name           string
		args           args
		wantCiphertext []byte
		wantTag        []byte
		wantErr        error
	}
	tests := []test{
		{
			name: "key size mismatch",
			args: args{
				algorithm: Algorithm_A128GCM,
				key:       []byte{0x00, 0x01},
				nonce:     mustDecodeHexString("cafebabefacedbaddecaf888"),
				plaintext: mustDecodeHexString("d9313225f88406e5a55909c5aff5269a86a7a9531534f7da2e4c303d8a318a721c3c0c95956809532fcf0e2449a6b525b16aedf5aa0de657ba637b391aafd255"),
			},
			wantErr: ErrKeyTypeMismatch,
		},
		{
			name: "nonce size mismatch",
			args: args{
				algorithm: Algorithm_A128CBC,
				key:       mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				nonce:     []byte{0x00, 0x01},
				plaintext: mustDecodeHexString("d9313225f88406e5a55909c5aff5269a86a7a9531534f7da2e4c303d8a318a721c3c0c95956809532fcf0e2449a6b525b16aedf5aa0de657ba637b391aafd255"),
			},
			wantErr: ErrInvalidNonce,
		},
	}

	// Test vectors from NIST publication SP800-38d
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-gcm") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm:      v.Algorithm,
				key:            mustDecodeHexString(v.Key),
				nonce:          mustDecodeHexString(v.Nonce),
				plaintext:      mustDecodeHexString(v.Plaintext),
				associatedData: mustDecodeHexString(v.AssociatedData),
			},
			wantCiphertext: mustDecodeHexString(v.Ciphertext),
			wantTag:        mustDecodeHexString(v.Tag),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCiphertext, gotTag, err := encryptSymmetricAESGCM(tt.args.plaintext, tt.args.algorithm, tt.args.key, tt.args.nonce, tt.args.associatedData)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("encryptSymmetricAESGCM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCiphertext, tt.wantCiphertext) {
				t.Errorf("encryptSymmetricAESGCM() gotCiphertext = %v, want %v", gotCiphertext, tt.wantCiphertext)
			}
			if !reflect.DeepEqual(gotTag, tt.wantTag) {
				t.Errorf("encryptSymmetricAESGCM() gotTag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}

func TestDecryptSymmetricAESGCM(t *testing.T) {
	type args struct {
		ciphertext     []byte
		algorithm      string
		key            []byte
		nonce          []byte
		tag            []byte
		associatedData []byte
	}
	type test struct {
		name          string
		args          args
		wantPlaintext []byte
		wantErr       error
	}
	tests := []test{
		{
			name: "key size mismatch",
			args: args{
				algorithm:  Algorithm_A128GCM,
				key:        []byte{0x00, 0x01},
				nonce:      mustDecodeHexString("cafebabefacedbaddecaf888"),
				ciphertext: mustDecodeHexString("42831ec2217774244b7221b784d0d49ce3aa212f2c02a4e035c17e2329aca12e21d514b25466931c7d8f6a5aac84aa051ba30b396a0aac973d58e091473f5985"),
				tag:        mustDecodeHexString("4d5c2af327cd64a62cf35abd2ba6fab4"),
			},
			wantErr: ErrKeyTypeMismatch,
		},
		{
			name: "nonce size mismatch",
			args: args{
				algorithm:  Algorithm_A128CBC,
				key:        mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				nonce:      []byte{0x00, 0x01},
				ciphertext: mustDecodeHexString("42831ec2217774244b7221b784d0d49ce3aa212f2c02a4e035c17e2329aca12e21d514b25466931c7d8f6a5aac84aa051ba30b396a0aac973d58e091473f5985"),
				tag:        mustDecodeHexString("4d5c2af327cd64a62cf35abd2ba6fab4"),
			},
			wantErr: ErrInvalidNonce,
		},
		{
			name: "tag size mismatch",
			args: args{
				algorithm:  Algorithm_A128CBC,
				key:        mustDecodeHexString("2b7e151628aed2a6abf7158809cf4f3c"),
				nonce:      mustDecodeHexString("cafebabefacedbaddecaf888"),
				ciphertext: mustDecodeHexString("42831ec2217774244b7221b784d0d49ce3aa212f2c02a4e035c17e2329aca12e21d514b25466931c7d8f6a5aac84aa051ba30b396a0aac973d58e091473f5985"),
				tag:        []byte{0x00, 0x01},
			},
			wantErr: ErrInvalidTag,
		},
	}

	// Test vectors from NIST publication SP800-38d
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-gcm") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm:      v.Algorithm,
				key:            mustDecodeHexString(v.Key),
				nonce:          mustDecodeHexString(v.Nonce),
				ciphertext:     mustDecodeHexString(v.Ciphertext),
				tag:            mustDecodeHexString(v.Tag),
				associatedData: mustDecodeHexString(v.AssociatedData),
			},
			wantPlaintext: mustDecodeHexString(v.Plaintext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPlaintext, err := decryptSymmetricAESGCM(tt.args.ciphertext, tt.args.algorithm, tt.args.key, tt.args.nonce, tt.args.tag, tt.args.associatedData)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("decryptSymmetricAESGCM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPlaintext, tt.wantPlaintext) && len(gotPlaintext) != 0 && len(tt.wantPlaintext) != 0 {
				t.Errorf("decryptSymmetricAESGCM() = %v, want %v", gotPlaintext, tt.wantPlaintext)
			}
		})
	}
}

func TestEncryptSymmetricAESKW(t *testing.T) {
	type args struct {
		plaintext []byte
		algorithm string
		key       []byte
	}
	type test struct {
		name           string
		args           args
		wantCiphertext []byte
		wantErr        error
	}
	tests := []test{}

	// Test cases from RFC3394
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-kw") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm: v.Algorithm,
				key:       mustDecodeHexString(v.Key),
				plaintext: mustDecodeHexString(v.Plaintext),
			},
			wantCiphertext: mustDecodeHexString(v.Ciphertext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCiphertext, err := encryptSymmetricAESKW(tt.args.plaintext, tt.args.algorithm, tt.args.key)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("encryptSymmetricAESKW() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCiphertext, tt.wantCiphertext) {
				t.Errorf("encryptSymmetricAESKW() = %v, want %v", gotCiphertext, tt.wantCiphertext)
			}
		})
	}
}

func TestDecryptSymmetricAESKW(t *testing.T) {
	type args struct {
		ciphertext []byte
		algorithm  string
		key        []byte
	}
	type test struct {
		name          string
		args          args
		wantPlaintext []byte
		wantErr       error
	}
	tests := []test{}

	// Test cases from RFC3394
	for _, v := range readTestVectors("symmetric-test-vectors.json", "aes-kw") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm:  v.Algorithm,
				key:        mustDecodeHexString(v.Key),
				ciphertext: mustDecodeHexString(v.Ciphertext),
			},
			wantPlaintext: mustDecodeHexString(v.Plaintext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPlaintext, err := decryptSymmetricAESKW(tt.args.ciphertext, tt.args.algorithm, tt.args.key)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("decryptSymmetricAESKW() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPlaintext, tt.wantPlaintext) {
				t.Errorf("decryptSymmetricAESKW() = %v, want %v", gotPlaintext, tt.wantPlaintext)
			}
		})
	}
}

func TestEncryptSymmetricChaCha20Poly1305(t *testing.T) {
	type args struct {
		plaintext      []byte
		algorithm      string
		key            []byte
		nonce          []byte
		associatedData []byte
	}
	type test struct {
		name           string
		args           args
		wantCiphertext []byte
		wantTag        []byte
		wantErr        error
	}
	tests := []test{}

	for _, v := range readTestVectors("symmetric-test-vectors.json", "chacha20-poly1305") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm:      v.Algorithm,
				key:            mustDecodeHexString(v.Key),
				nonce:          mustDecodeHexString(v.Nonce),
				plaintext:      mustDecodeHexString(v.Plaintext),
				associatedData: mustDecodeHexString(v.AssociatedData),
			},
			wantCiphertext: mustDecodeHexString(v.Ciphertext),
			wantTag:        mustDecodeHexString(v.Tag),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCiphertext, gotTag, err := encryptSymmetricChaCha20Poly1305(tt.args.plaintext, tt.args.algorithm, tt.args.key, tt.args.nonce, tt.args.associatedData)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("encryptSymmetricChaCha20Poly1305() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCiphertext, tt.wantCiphertext) {
				t.Errorf("encryptSymmetricChaCha20Poly1305() gotCiphertext = %v, want %v", gotCiphertext, tt.wantCiphertext)
			}
			if !reflect.DeepEqual(gotTag, tt.wantTag) {
				t.Errorf("encryptSymmetricChaCha20Poly1305() gotTag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}

func TestDecryptSymmetricChaCha20Poly1305(t *testing.T) {
	type args struct {
		ciphertext     []byte
		algorithm      string
		key            []byte
		nonce          []byte
		tag            []byte
		associatedData []byte
	}
	type test struct {
		name          string
		args          args
		wantPlaintext []byte
		wantErr       error
	}
	tests := []test{}

	for _, v := range readTestVectors("symmetric-test-vectors.json", "chacha20-poly1305") {
		tests = append(tests, test{
			name: v.Name,
			args: args{
				algorithm:      v.Algorithm,
				key:            mustDecodeHexString(v.Key),
				nonce:          mustDecodeHexString(v.Nonce),
				ciphertext:     mustDecodeHexString(v.Ciphertext),
				tag:            mustDecodeHexString(v.Tag),
				associatedData: mustDecodeHexString(v.AssociatedData),
			},
			wantPlaintext: mustDecodeHexString(v.Plaintext),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPlaintext, err := decryptSymmetricChaCha20Poly1305(tt.args.ciphertext, tt.args.algorithm, tt.args.key, tt.args.nonce, tt.args.tag, tt.args.associatedData)
			if ((err != nil) != (tt.wantErr != nil)) ||
				(err != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("decryptSymmetricChaCha20Poly1305() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPlaintext, tt.wantPlaintext) && len(gotPlaintext) != 0 && len(tt.wantPlaintext) != 0 {
				t.Errorf("decryptSymmetricChaCha20Poly1305() = %v, want %v", gotPlaintext, tt.wantPlaintext)
			}
		})
	}
}

func TestAESCBCHMAC(t *testing.T) {
	plaintext := mustDecodeHexString("41206369706865722073797374656d206d757374206e6f7420626520726571756972656420746f206265207365637265742c20616e64206974206d7573742062652061626c6520746f2066616c6c20696e746f207468652068616e6473206f662074686520656e656d7920776974686f757420696e636f6e76656e69656e6365")
	nonce := mustDecodeHexString("1af38c2dc2b96ffdd86694092341bc04")
	aad := mustDecodeHexString("546865207365636f6e64207072696e6369706c65206f662041756775737465204b6572636b686f666673")

	type test struct {
		alg        string
		key        []byte
		ciphertext []byte
		tag        []byte
	}
	tests := []test{
		{
			alg:        Algorithm_A128CBC_HS256,
			key:        mustDecodeHexString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
			ciphertext: mustDecodeHexString("c80edfa32ddf39d5ef00c0b468834279a2e46a1b8049f792f76bfe54b903a9c9a94ac9b47ad2655c5f10f9aef71427e2fc6f9b3f399a221489f16362c703233609d45ac69864e3321cf82935ac4096c86e133314c54019e8ca7980dfa4b9cf1b384c486f3a54c51078158ee5d79de59fbd34d848b3d69550a67646344427ade54b8851ffb598f7f80074b9473c82e2db"),
			tag:        mustDecodeHexString("652c3fa36b0a7c5b3219fab3a30bc1c4"),
		},
		{
			alg:        Algorithm_A192CBC_HS384,
			key:        mustDecodeHexString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f"),
			ciphertext: mustDecodeHexString("ea65da6b59e61edb419be62d19712ae5d303eeb50052d0dfd6697f77224c8edb000d279bdc14c1072654bd30944230c657bed4ca0c9f4a8466f22b226d1746214bf8cfc2400add9f5126e479663fc90b3bed787a2f0ffcbf3904be2a641d5c2105bfe591bae23b1d7449e532eef60a9ac8bb6c6b01d35d49787bcd57ef484927f280adc91ac0c4e79c7b11efc60054e3"),
			tag:        mustDecodeHexString("8490ac0e58949bfe51875d733f93ac2075168039ccc733d7"),
		},
		{
			alg:        Algorithm_A256CBC_HS512,
			key:        mustDecodeHexString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"),
			ciphertext: mustDecodeHexString("4affaaadb78c31c5da4b1b590d10ffbd3dd8d5d302423526912da037ecbcc7bd822c301dd67c373bccb584ad3e9279c2e6d12a1374b77f077553df829410446b36ebd97066296ae6427ea75c2e0846a11a09ccf5370dc80bfecbad28c73f09b3a3b75e662a2594410ae496b2e2e6609e31e6e02cc837f053d21f37ff4f51950bbe2638d09dd7a4930930806d0703b1f6"),
			tag:        mustDecodeHexString("4dd3b4c088a7f45c216839645b2012bf2e6269a8c56a816dbc1b267761955bc5"),
		},
	}

	for _, tt := range tests {
		t.Run("alg "+tt.alg, func(t *testing.T) {
			// Compare with the ciphertext with AES-CBC without HMAC
			cbcKeySize := expectedKeySize(tt.alg[0:7])
			gotCiphertext, err := encryptSymmetricAESCBC(plaintext, tt.alg[0:7], tt.key[len(tt.key)-cbcKeySize:], nonce)
			require.NoError(t, err)
			assert.Equal(t, tt.ciphertext, gotCiphertext)

			// AEAD: Encrypt with AES-CBC and HMAC-SHA
			gotCiphertext, gotTag, err := encryptSymmetricAESCBCHMAC(plaintext, tt.alg, tt.key, nonce, aad)
			require.NoError(t, err)
			assert.Equal(t, tt.ciphertext, gotCiphertext)
			assert.Equal(t, tt.tag, gotTag)

			// Decrypt back
			gotPlaintext, err := decryptSymmetricAESCBCHMAC(gotCiphertext, tt.alg, tt.key, nonce, gotTag, aad)
			require.NoError(t, err)
			assert.Equal(t, plaintext, gotPlaintext)
		})
	}
}

type testVector struct {
	Name           string `json:"name"`
	Algorithm      string `json:"algorithm"`
	Key            string `json:"key"`
	Nonce          string `json:"nonce"`
	AssociatedData string `json:"associatedData"`
	Plaintext      string `json:"plaintext"`
	Ciphertext     string `json:"ciphertext"`
	Tag            string `json:"tag"`
}

func readTestVectors(fileName string, vectorsName string) []testVector {
	f, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	read := map[string][]testVector{}
	err = json.NewDecoder(f).Decode(&read)
	if err != nil {
		panic(err)
	}

	return read[vectorsName]
}

func mustDecodeHexString(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
