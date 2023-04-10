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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/require"
)

var (
	errSimulatedStream = errors.New("simulated stream error")
	errSimulated       = errors.New("simulated")
)

func TestScheme(t *testing.T) {
	// Fake wrapKeyFn and unwrapKeyFn, which just return the plaintext key
	//nolint:stylecheck
	var wrapKeyFn WrapKeyFn = func(plaintextKey jwk.Key, algorithm, keyName string, nonce []byte) (wrappedKey []byte, tag []byte, err error) {
		err = plaintextKey.Raw(&wrappedKey)
		return
	}
	//nolint:stylecheck
	var unwrapKeyFn UnwrapKeyFn = func(wrappedKey []byte, algorithm, keyName string, nonce, tag []byte) (plaintextKey jwk.Key, err error) {
		return jwk.FromRaw(wrappedKey)
	}

	// In all these tests, the key name and wrapping algorithms don't matter as we don't actually wrap/unwrap keys
	const keyName = "mykey"
	const algorithm = KeyAlgorithmAES

	testData := map[string][]byte{
		// Data is short and fits in a single segment
		"single-segment": []byte("hello world"),
		// Data is larger than a single segment (120KB)
		"multi-segment": bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}, 12<<10),
		// Data is exactly the size of a segment (64KB)
		"one-full-segment": bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8}, 8<<10),
		// Data is exactly the size of two segments (128KB)
		"two-full-segments": bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8}, 16<<10),
		// Large file (300KB)
		"large-file": bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}, 30<<10),
		// Empty message - this should succeed
		"empty-message": {},
	}

	t.Run("encrypt and decrypt", func(t *testing.T) {
		testFn := func(message []byte, cipher Cipher) func(t *testing.T) {
			return func(t *testing.T) {
				// Encrypt the message
				enc, err := Encrypt(
					bytes.NewReader(message),
					EncryptOptions{
						WrapKeyFn: wrapKeyFn,
						KeyName:   keyName,
						Algorithm: algorithm,
						Cipher:    &cipher,
					},
				)
				require.NoError(t, err)

				// Read the encrypted data
				encData, err := io.ReadAll(enc)
				require.NoError(t, err)
				require.NotEmpty(t, encData)

				// Sanity check of the header
				// First, ensure the scheme name is there
				idx := bytes.IndexByte(encData, '\n')
				require.Equal(t, 14, idx)
				require.Equal(t, SchemeName, string(encData[0:idx]))

				// Second, check that the JSON manifest is present and valid
				start := idx + 1
				idx = bytes.IndexByte(encData[start:], '\n')
				require.Greater(t, idx, 0)
				var manifest Manifest
				err = json.Unmarshal(encData[start:(start+idx)], &manifest)
				require.NoError(t, err)
				require.NoError(t, manifest.Validate())
				require.Equal(t, keyName, manifest.KeyName)
				require.Equal(t, algorithm.ID(), manifest.KeyWrappingAlgorithm.ID())
				require.Equal(t, cipher.ID(), manifest.Cipher.ID())
				require.Len(t, manifest.WFK, 32)
				require.Len(t, manifest.NoncePrefix, 7)

				// Third, check that we have the MAC
				// We are not validating the MAC here as the decryption code will do it; we'll just check it's present and 44-byte long (when encoded as base64)
				start += idx + 1
				idx = bytes.IndexByte(encData[start:], '\n')
				require.Greater(t, idx, 0)
				require.Len(t, encData[start:(start+idx)], 44)

				// Decrypt the encrypted data
				dec, err := Decrypt(
					bytes.NewReader(encData),
					DecryptOptions{
						UnwrapKeyFn: unwrapKeyFn,
					},
				)
				require.NoError(t, err)

				// The encrypted data should match
				decData, err := io.ReadAll(dec)
				require.NoError(t, err)
				require.Equal(t, message, decData)
			}
		}

		testFnAllCiphers := func(message []byte) func(t *testing.T) {
			return func(t *testing.T) {
				t.Run("with AES-GCM", testFn(message, CipherAESGCM))
				t.Run("with ChaCha20-Poly1305", testFn(message, CipherChaCha20Poly1305))
			}
		}

		t.Run("single-segment", testFnAllCiphers(testData["single-segment"]))
		t.Run("multi-segment", testFnAllCiphers(testData["multi-segment"]))
		t.Run("one-full-segment", testFnAllCiphers(testData["one-full-segment"]))
		t.Run("two-full-segments", testFnAllCiphers(testData["two-full-segments"]))
		t.Run("large-file", testFnAllCiphers(testData["large-file"]))
		t.Run("empty-message", testFnAllCiphers(testData["empty-message"]))
	})

	t.Run("decrypt test data", func(t *testing.T) {
		testFn := func(fileName string, expectData []byte) func(t *testing.T) {
			return func(t *testing.T) {
				enc, err := os.Open(filepath.Join("testdata", fileName))
				require.NoError(t, err)
				defer enc.Close()

				// Decrypt the encrypted data
				dec, err := Decrypt(
					enc,
					DecryptOptions{
						UnwrapKeyFn: unwrapKeyFn,
					},
				)
				require.NoError(t, err)

				// The encrypted data should match
				decData, err := io.ReadAll(dec)
				require.NoError(t, err)
				require.Equal(t, expectData, decData)
			}
		}

		t.Run("single-segment", testFn("single-segment.enc", testData["single-segment"]))
		t.Run("multi-segment", testFn("multi-segment.enc", testData["multi-segment"]))
		t.Run("one-full-segment", testFn("one-full-segment.enc", testData["one-full-segment"]))
		t.Run("two-full-segments", testFn("two-full-segments.enc", testData["two-full-segments"]))
		t.Run("empty-message", testFn("empty-message.enc", testData["empty-message"]))
	})

	t.Run("default cipher in encryption is AES-GCM", func(t *testing.T) {
		// Encrypt the message
		enc, err := Encrypt(
			strings.NewReader("hello world"),
			EncryptOptions{
				WrapKeyFn: wrapKeyFn,
				KeyName:   keyName,
				Algorithm: algorithm,
				// Explicitly set to nil
				Cipher: nil,
			},
		)
		require.NoError(t, err)

		// Read the encrypted data
		encData, err := io.ReadAll(enc)
		require.NoError(t, err)
		require.NotEmpty(t, encData)

		// Get the JSON manifest
		start := bytes.IndexByte(encData, '{')
		require.Greater(t, start, 14)
		end := start + bytes.IndexByte(encData[start:], '\n')
		require.Greater(t, end, start)
		var manifest Manifest
		err = json.Unmarshal(encData[start:end], &manifest)
		require.NoError(t, err)
		require.NoError(t, manifest.Validate())
		require.Equal(t, CipherAESGCM.ID(), manifest.Cipher.ID())
		require.Len(t, manifest.WFK, 32)
		require.Len(t, manifest.NoncePrefix, 7)
	})

	t.Run("encryption option DecryptionKeyName", func(t *testing.T) {
		// Encrypt the message
		enc, err := Encrypt(
			bytes.NewReader(testData["single-segment"]),
			EncryptOptions{
				WrapKeyFn:         wrapKeyFn,
				KeyName:           keyName,
				Algorithm:         algorithm,
				DecryptionKeyName: "dec-key",
			},
		)
		require.NoError(t, err)

		// Read the encrypted data
		encData, err := io.ReadAll(enc)
		require.NoError(t, err)
		require.NotEmpty(t, encData)

		// Get the JSON manifest
		start := bytes.IndexByte(encData, '{')
		require.Greater(t, start, 14)
		end := start + bytes.IndexByte(encData[start:], '\n')
		require.Greater(t, end, start)
		var manifest Manifest
		err = json.Unmarshal(encData[start:end], &manifest)
		require.NoError(t, err)
		require.NoError(t, manifest.Validate())
		require.Equal(t, "dec-key", manifest.KeyName)
	})

	t.Run("encryption option OmitKeyName", func(t *testing.T) {
		// Encrypt the message
		enc, err := Encrypt(
			bytes.NewReader(testData["single-segment"]),
			EncryptOptions{
				WrapKeyFn:         wrapKeyFn,
				KeyName:           keyName,
				Algorithm:         algorithm,
				DecryptionKeyName: "dec-key", // Should be ignored
				OmitKeyName:       true,
			},
		)
		require.NoError(t, err)

		// Read the encrypted data
		encData, err := io.ReadAll(enc)
		require.NoError(t, err)
		require.NotEmpty(t, encData)

		// Get the JSON manifest
		start := bytes.IndexByte(encData, '{')
		require.Greater(t, start, 14)
		end := start + bytes.IndexByte(encData[start:], '\n')
		require.Greater(t, end, start)
		var manifest Manifest
		err = json.Unmarshal(encData[start:end], &manifest)
		require.NoError(t, err)
		require.NoError(t, manifest.Validate())
		require.Empty(t, manifest.KeyName)
	})

	t.Run("decryption of a message created with OmitKeyName requires passing a key name", func(t *testing.T) {
		enc, err := os.Open(filepath.Join("testdata", "single-segment-no-key-name.enc"))
		require.NoError(t, err)
		defer enc.Close()

		// Decryption requires passing the key name
		dec, err := Decrypt(
			enc,
			DecryptOptions{
				KeyName:     "mykey",
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.NoError(t, err)

		// The encrypted data should match
		decData, err := io.ReadAll(dec)
		require.NoError(t, err)
		require.Equal(t, testData["single-segment"], decData)
	})

	t.Run("decryption of a message created with OmitKeyName fails without a key name", func(t *testing.T) {
		enc, err := os.Open(filepath.Join("testdata", "single-segment-no-key-name.enc"))
		require.NoError(t, err)
		defer enc.Close()

		// Do not pass a key name
		dec, err := Decrypt(
			enc,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDecryptionKeyMissing)
		require.Nil(t, dec)
	})

	t.Run("wrapKeyFn receives the key name and algorithm", func(t *testing.T) {
		var (
			gotKeyName   string
			gotAlgorithm string
		)
		_, err := Encrypt(
			strings.NewReader("hello world"),
			EncryptOptions{
				WrapKeyFn: func(plaintextKey jwk.Key, algorithm, keyName string, nonce []byte) (wrappedKey []byte, tag []byte, err error) {
					gotAlgorithm = algorithm
					gotKeyName = keyName
					return wrapKeyFn(plaintextKey, algorithm, keyName, nonce)
				},
				// The actual values don't matter in this test
				KeyName:   "fakekey",
				Algorithm: KeyAlgorithmRSAOAEP256,
				// Explicitly set to nil
				Cipher: nil,
			},
		)
		require.NoError(t, err)

		require.Equal(t, "fakekey", gotKeyName)
		require.Equal(t, string(KeyAlgorithmRSAOAEP256), gotAlgorithm)
	})

	t.Run("override key name in decryption", func(t *testing.T) {
		enc, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer enc.Close()

		// Decrypt the encrypted data
		var gotKeyName string
		dec, err := Decrypt(
			enc,
			DecryptOptions{
				// Although we're passing a different value for keyName, we still return the same key so decryption will work
				KeyName: "anotherkey",
				UnwrapKeyFn: func(wrappedKey []byte, algorithm, keyName string, nonce, tag []byte) (plaintextKey jwk.Key, err error) {
					gotKeyName = keyName
					return unwrapKeyFn(wrappedKey, algorithm, keyName, nonce, tag)
				},
			},
		)
		require.NoError(t, err)

		// The encrypted data should match
		decData, err := io.ReadAll(dec)
		require.NoError(t, err)
		require.Equal(t, testData["single-segment"], decData)

		// The key name should be "anotherkey"
		require.Equal(t, "anotherkey", gotKeyName)
	})

	t.Run("encryption fails with input stream error", func(t *testing.T) {
		enc, err := Encrypt(
			&failingReader{},
			EncryptOptions{
				WrapKeyFn: wrapKeyFn,
				KeyName:   keyName,
				Algorithm: algorithm,
			},
		)
		require.NoError(t, err)

		// Read the encrypted data
		_, err = io.ReadAll(enc)
		require.Error(t, err)
		require.ErrorIs(t, err, errSimulatedStream)
	})

	t.Run("wrapping key fails in Encrypt", func(t *testing.T) {
		enc, err := Encrypt(
			&bytes.Buffer{},
			EncryptOptions{
				WrapKeyFn: func(plaintextKey jwk.Key, algorithm, keyName string, nonce []byte) (wrappedKey []byte, tag []byte, err error) {
					return nil, nil, errSimulated
				},
				KeyName:   keyName,
				Algorithm: algorithm,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to wrap the file key")
		require.Nil(t, enc)
	})

	t.Run("unwrapping key fails in Decrypt", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		// When the wrapping function returns an error, that is swallowed and the user will only see "failed to validate the document's signature"
		// That's by design
		dec, err := Decrypt(
			f,
			DecryptOptions{
				UnwrapKeyFn: func(wrappedKey []byte, algorithm, keyName string, nonce, tag []byte) (plaintextKey jwk.Key, err error) {
					return nil, errSimulated
				},
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to validate the document's signature")
		require.Nil(t, dec)
	})

	t.Run("unwrapping key returns different key in Decrypt", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		dec, err := Decrypt(
			f,
			DecryptOptions{
				UnwrapKeyFn: func(wrappedKey []byte, algorithm, keyName string, nonce, tag []byte) (plaintextKey jwk.Key, err error) {
					return jwk.FromRaw(bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8}, 4))
				},
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to validate the document's signature")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with scheme name not found", func(t *testing.T) {
		dec, err := Decrypt(
			strings.NewReader("foo"),
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: scheme name not found")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with scheme name not matching", func(t *testing.T) {
		dec, err := Decrypt(
			strings.NewReader("invalidscheme\nfoo"),
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: unsupported scheme")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with manifest not found", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		dec, err := Decrypt(
			&failingReader{data: io.LimitReader(f, 20)},
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: manifest not found")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with manifest zero bytes", func(t *testing.T) {
		dec, err := Decrypt(
			strings.NewReader("dapr.io/enc/v1\n\n"),
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: invalid format")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with manifest not valid JSON", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		// This manifest will not unmarshal as JSON
		rr := newReplaceReader(f, 15, 116, strings.NewReader("notjson"))

		dec, err := Decrypt(
			rr,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: invalid manifest")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with manifest not validating", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		// This manifest will unmarshal into the Manifest struct, but will fail the Validate() method
		// We won't test all possible violations here because they're tested in the manifest_test.go file
		rr := newReplaceReader(f, 15, 116, strings.NewReader(`{"wk":"foo"}`))

		dec, err := Decrypt(
			rr,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: invalid manifest")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with MAC not found", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		dec, err := Decrypt(
			&failingReader{data: io.LimitReader(f, 120)},
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: message authentication code not found")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with MAC zero bytes", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		in := io.MultiReader(
			io.LimitReader(f, 117),
			bytes.NewReader([]byte{'\n'}),
		)
		dec, err := Decrypt(
			&failingReader{data: in},
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: invalid format")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with MAC not valid Base64", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		// Replace some bytes in the MAC
		rr := newReplaceReader(f, 120, 121, strings.NewReader("*"))

		dec, err := Decrypt(
			rr,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to decode header's signature: illegal base64 data")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with header too long", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		// After the manifest (included in the first 120 bytes), add 80KB of zeros
		in := io.MultiReader(
			io.LimitReader(f, 120),
			bytes.NewReader(bytes.Repeat([]byte{0}, 120<<10)),
		)
		dec, err := Decrypt(
			in,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid header: message authentication code not found")
		require.Nil(t, dec)
	})

	t.Run("decryption fails with input stream error after header", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "single-segment.enc"))
		require.NoError(t, err)
		defer f.Close()

		dec, err := Decrypt(
			&failingReader{data: f},
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.NoError(t, err)

		_, err = io.ReadAll(dec)
		require.Error(t, err)
		require.ErrorIs(t, err, errSimulatedStream)
	})

	t.Run("decryption fails when a byte is changed in the ciphertext", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "large-file.enc"))
		require.NoError(t, err)
		defer f.Close()

		// Replace a byte in the second segment (segment 1)
		rr := newReplaceReader(f, 100_000, 100_001, bytes.NewReader([]byte{'A'}))

		dec, err := Decrypt(
			rr,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.NoError(t, err)

		_, err = io.ReadAll(dec)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDecryptionFailed)
		require.ErrorContains(t, err, "error processing segment 1")
	})

	t.Run("decryption fails when a segment is removed from the ciphertext", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "large-file.enc"))
		require.NoError(t, err)
		defer f.Close()

		// Remove the third segment (segment 2)
		rr := newReplaceReader(f, 162+(SegmentSize+SegmentOverhead)*2, 162+(SegmentSize+SegmentOverhead)*3, &bytes.Buffer{})

		dec, err := Decrypt(
			rr,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.NoError(t, err)

		_, err = io.ReadAll(dec)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDecryptionFailed)
		require.ErrorContains(t, err, "error processing segment 2")
	})

	t.Run("decryption fails when the last segment is removed from the ciphertext", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "large-file.enc"))
		require.NoError(t, err)
		defer f.Close()

		// Remove the last segment (segment 4)
		// This will fail on segment 3 because at that point it becomes the last
		rr := newReplaceReader(f, 162+(SegmentSize+SegmentOverhead)*4, -1, &bytes.Buffer{})

		dec, err := Decrypt(
			rr,
			DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			},
		)
		require.NoError(t, err)

		_, err = io.ReadAll(dec)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDecryptionFailed)
		require.ErrorContains(t, err, "error processing segment 3")
	})

	t.Run("init errors for Encrypt", func(t *testing.T) {
		t.Run("input stream is nil", func(t *testing.T) {
			out, err := Encrypt(nil, EncryptOptions{
				WrapKeyFn: wrapKeyFn,
				KeyName:   keyName,
				Algorithm: algorithm,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "in stream is nil")
			require.Nil(t, out)
		})

		t.Run("option WrapKeyFn is empty", func(t *testing.T) {
			out, err := Encrypt(&bytes.Buffer{}, EncryptOptions{
				KeyName:   keyName,
				Algorithm: algorithm,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "option WrapKeyFn is required")
			require.Nil(t, out)
		})

		t.Run("option KeyName is empty", func(t *testing.T) {
			out, err := Encrypt(&bytes.Buffer{}, EncryptOptions{
				WrapKeyFn: wrapKeyFn,
				Algorithm: algorithm,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "option KeyName is required")
			require.Nil(t, out)
		})

		t.Run("option Algorithm is empty", func(t *testing.T) {
			out, err := Encrypt(&bytes.Buffer{}, EncryptOptions{
				WrapKeyFn: wrapKeyFn,
				KeyName:   keyName,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "option Algorithm is required")
			require.Nil(t, out)
		})

		t.Run("option Algorithm is invalid", func(t *testing.T) {
			out, err := Encrypt(&bytes.Buffer{}, EncryptOptions{
				WrapKeyFn: wrapKeyFn,
				KeyName:   keyName,
				Algorithm: "invalid",
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "option Algorithm is not valid")
			require.Nil(t, out)
		})

		t.Run("option Cipher is invalid", func(t *testing.T) {
			invalidCipher := Cipher("invalid")
			out, err := Encrypt(&bytes.Buffer{}, EncryptOptions{
				WrapKeyFn: wrapKeyFn,
				KeyName:   keyName,
				Algorithm: algorithm,
				Cipher:    &invalidCipher,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "option Cipher is not valid")
			require.Nil(t, out)
		})
	})

	t.Run("init errors for Decrypt", func(t *testing.T) {
		t.Run("input stream is nil", func(t *testing.T) {
			out, err := Decrypt(nil, DecryptOptions{
				UnwrapKeyFn: unwrapKeyFn,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, "in stream is nil")
			require.Nil(t, out)
		})

		t.Run("option UnwrapKeyFn is empty", func(t *testing.T) {
			out, err := Decrypt(&bytes.Buffer{}, DecryptOptions{})
			require.Error(t, err)
			require.ErrorContains(t, err, "option UnwrapKeyFn is required")
			require.Nil(t, out)
		})
	})
}

func TestReplaceReader(t *testing.T) {
	const message = "Ho sceso, dandoti il braccio, almeno un milione di scale e ora che non ci sei è il vuoto ad ogni gradino."

	t.Run("replace bytes", func(t *testing.T) {
		const replace = "✂️"
		const expect = "Ho sceso, dandoti il braccio, almeno un milione di scale e ora✂️è il vuoto ad ogni gradino."

		rr := newReplaceReader(strings.NewReader(message), 62, 78, strings.NewReader(replace))
		read, err := io.ReadAll(rr)
		require.NoError(t, err)
		require.Equal(t, expect, string(read))
	})

	t.Run("remove bytes", func(t *testing.T) {
		const expect = "Ho sceso, dandoti il braccio, almeno un milione di scale e ora è il vuoto ad ogni gradino."

		rr := newReplaceReader(strings.NewReader(message), 63, 78, &bytes.Buffer{})
		read, err := io.ReadAll(rr)
		require.NoError(t, err)
		require.Equal(t, expect, string(read))
	})

	t.Run("remove at the end", func(t *testing.T) {
		const expect = "Ho sceso, dandoti il braccio, almeno un milione di scale e ora che non ci sei è il vuoto"

		rr := newReplaceReader(strings.NewReader(message), 89, -1, &bytes.Buffer{})
		read, err := io.ReadAll(rr)
		require.NoError(t, err)
		require.Equal(t, expect, string(read))
	})
}

// Implements an io.Reader that replaces a segment in the stream with custom data
type replaceReader struct {
	stream   io.Reader
	cutStart int
	cutEnd   int // If -1, removes till the ned
	replace  io.Reader

	// Internal properties
	read      int
	replacing bool
	l         sync.Mutex
}

func newReplaceReader(stream io.Reader, cutStart, cutEnd int, replace io.Reader) io.Reader {
	return &replaceReader{
		stream:   stream,
		cutStart: cutStart,
		cutEnd:   cutEnd,
		replace:  replace,
	}
}

func (r *replaceReader) Read(p []byte) (int, error) {
	if r.cutEnd == 0 || (r.cutEnd > 0 && r.cutStart > r.cutEnd) {
		panic("cutStart and/or cutEnd are not valid")
	}

	r.l.Lock()
	defer r.l.Unlock()

	// If we've already replaced the data and there's no more data left, just read from the rest of the stream
	if r.replacing && r.replace == nil {
		return r.stream.Read(p)
	}

	// If we're in the replacement section, read from the replace stream
	if r.replacing {
		n, err := r.replace.Read(p)
		if errors.Is(err, io.EOF) {
			err = nil
			r.replace = nil
		}
		return n, err
	}

	max := len(p)
	if (max + r.read) > r.cutStart {
		max = r.cutStart - r.read
	}
	n, err := r.stream.Read(p[:max])
	r.read += n

	if r.read >= r.cutStart {
		// Advance the stream till the cut end, ignoring errors
		if r.cutEnd < 0 {
			io.Copy(io.Discard, r.stream)
		} else {
			io.CopyN(io.Discard, r.stream, int64(r.cutEnd-r.cutStart))
		}
		r.replacing = true
	}

	return n, err
}

// Implements an io.Reader that simulates failures (after optionally reading from a stream in full)
type failingReader struct {
	// Data to return before returning an error
	data io.Reader
	l    sync.Mutex
}

func (f *failingReader) Read(p []byte) (n int, err error) {
	f.l.Lock()
	defer f.l.Unlock()

	if f.data != nil {
		n, err := f.data.Read(p)
		if err == nil {
			return n, nil
		} else if errors.Is(err, io.EOF) {
			// Do not return io.EOF as error
			// Instead, just delete the stream
			// On the next call, we will return an error
			f.data = nil
			return n, nil
		} else {
			// Should not happen
			panic(err)
		}
	}

	return 0, errSimulatedStream
}
