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
	"fmt"
	"io"
	"sync"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

const (
	// SchemeName is the name of the encryption scheme.
	SchemeName = "dapr.io/enc/v1"

	// Size of each segment in the encrypted message.
	// Each segment is exactly 64KB in length, except the last one which could be shorter.
	SegmentSize = 64 << 10

	// Overhead of each segment in bytes
	// This is equivalent to the size of the authentication tag for AES-GCM and ChaCha20-Poly1305
	SegmentOverhead = 16
)

var (
	// Error returned when trying to decrypt a document whose manifest does not contain a key name, and the caller did not provide an explicit key name.
	ErrDecryptionKeyMissing = errors.New("document's manifest does not contain a key name, and no key name was provided")

	// Error returned when the signature of the document could not be validated.
	ErrDecryptionSignature = errors.New("failed to validate the document's signature")

	// Error returned when the deryption fails.
	// Most commonly this happens when a segment has been tampered with.
	ErrDecryptionFailed = errors.New("failed to decrypt segment")
)

type (
	// Signature of the method that wraps keys.
	// This does not accept a context, which needs to be provided by the caller of the Encrypt method inside the lambda.
	WrapKeyFn = func(plaintextKey jwk.Key, algorithm string, keyName string, nonce []byte) (wrappedKey []byte, tag []byte, err error)

	// Signature of the method that unwraps keys.
	// This does not accept a context, which needs to be provided by the caller of the Decrypt method inside the lambda.
	UnwrapKeyFn = func(wrappedKey []byte, algorithm string, keyName string, nonce []byte, tag []byte) (plaintextKey jwk.Key, err error)
)

// EncryptOptions contains the options passed to the Encrypt method
type EncryptOptions struct {
	// Function that is invoked to wrap the key
	WrapKeyFn WrapKeyFn
	// Algorithm used to wrap the file key
	// This must be one of the supported KeyAlgorithm constants, and must be usable by the kind of key provided
	Algorithm KeyAlgorithm
	// Name of the key to use
	KeyName string
	// Name of the key to include as decryption key
	// If empty, uses KeyName
	DecryptionKeyName string
	// If true, does not include the key name in the manifest
	OmitKeyName bool
	// Cipher used to encrypt the data
	// If nil, defaults to AES-GCM
	Cipher *Cipher
}

// DecryptOptions contains the options passed to the Decrypt method
type DecryptOptions struct {
	// Function that is invoked to unwrap the key
	UnwrapKeyFn UnwrapKeyFn
	// If set, uses this value as key name rather than the one included in the manifest
	KeyName string
}

// BufPool is a sync.Pool that returns buffers of SegmentSize+SegmentOverhead, plus one extra byte
var BufPool = sync.Pool{
	New: func() any {
		const bufSize = SegmentSize + SegmentOverhead + 1
		// Return a pointer here
		// See https://github.com/dominikh/go-tools/issues/1336 for explanation
		b := make([]byte, bufSize)
		return &b
	},
}

// Encrypt a document using the `dapr.io/enc/v1` scheme.
// The plaintext is read from the `in` stream and written to the returned stream.
func Encrypt(in io.Reader, opts EncryptOptions) (io.Reader, error) {
	// Validate the request options
	if in == nil {
		return nil, errors.New("in stream is nil")
	}
	if opts.WrapKeyFn == nil {
		return nil, errors.New("option WrapKeyFn is required")
	}
	if opts.KeyName == "" {
		return nil, errors.New("option KeyName is required")
	}
	if opts.Algorithm == "" {
		return nil, errors.New("option Algorithm is required")
	}
	keyWrapAlgorithm, err := opts.Algorithm.Validate()
	if err != nil {
		return nil, fmt.Errorf("option Algorithm is not valid: %w", err)
	}
	cipher := CipherAESGCM
	if opts.Cipher != nil {
		cipher, err = opts.Cipher.Validate()
		if err != nil {
			return nil, fmt.Errorf("option Cipher is not valid: %w", err)
		}
	}

	// Start by generating a random file key
	fk, err := newFileKey(cipher)
	if err != nil {
		return nil, err
	}

	// Wrap the file key
	// Note: we're skipping the nonce and ignoring the tag parameter at the moment because none of the supported ciphers use them
	fileKeyJWK, err := fk.GetKeyJWK()
	if err != nil {
		return nil, err
	}
	wrappedFileKey, _, err := opts.WrapKeyFn(fileKeyJWK, string(keyWrapAlgorithm), opts.KeyName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap the file key: %w", err)
	}

	// Create the manifest and sign it
	keyName := opts.DecryptionKeyName
	if opts.OmitKeyName {
		keyName = ""
	} else if keyName == "" {
		keyName = opts.KeyName
	}
	manifest, err := json.Marshal(&Manifest{
		KeyName:              keyName,
		KeyWrappingAlgorithm: keyWrapAlgorithm,
		WFK:                  wrappedFileKey,
		Cipher:               cipher,
		NoncePrefix:          fk.GetNoncePrefix(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JSON manifest: %w", err)
	}
	header, err := fk.SignHeader(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to sign header: %w", err)
	}

	// Start a background goroutine to perform the encryption, and return the stream to the caller
	// From now on, errors are returned as errors on the stream
	outR, outW := io.Pipe()
	go func() {
		// Write the header
		if !writeOrClosePipe(outW, header) {
			return
		}

		// Proceed with processing all segments
		processSegments(fk, in, outW, fk.EncryptSegment, SegmentSize)
	}()

	return outR, nil
}

// Decrypt a document using the `dapr.io/enc/v1` scheme
// The ciphertext is read from the `in` stream and written to the returned stream
func Decrypt(in io.Reader, opts DecryptOptions) (io.Reader, error) {
	// Validate the request options
	if in == nil {
		return nil, errors.New("in stream is nil")
	}
	if opts.UnwrapKeyFn == nil {
		return nil, errors.New("option UnwrapKeyFn is required")
	}

	// Read the header
	manifest, mac, err := readHeader(&in)
	if err != nil {
		return nil, fmt.Errorf("invalid header: %w", err)
	}

	// Parse the manifest to get the key name and validate it
	var manifestObj Manifest
	err = json.Unmarshal(manifest, &manifestObj)
	if err != nil || manifestObj.Validate() != nil {
		// Do not return the exact error to avoid disclosing too much information
		return nil, errors.New("invalid header: invalid manifest")
	}

	// Get the name of the key, and check if we need to override it
	keyName := opts.KeyName
	if keyName == "" {
		keyName = manifestObj.KeyName
		if keyName == "" {
			return nil, ErrDecryptionKeyMissing
		}
	}

	// Unwrap the file key
	// Note: we're skipping the nonce and tag parameters at the moment because none of the supported ciphers use them
	var fileKeyBytes []byte
	fileKeyJWK, err := opts.UnwrapKeyFn(manifestObj.WFK, string(manifestObj.KeyWrappingAlgorithm), keyName, nil, nil)
	if err != nil {
		// This is where things get a bit tricky.
		// If the UnwrapKeyFn returned an error, we want to ignore that for now, and instead continue validating the MAC using an empty fileKey (which will fail).
		// This is because otherwise we may be making it easier to disclose certain information such as whether a key exists or not in the vault via timing attacks.
		// What we're doing here doesn't remove timing attacks entirely, starting from the fact that we're putting an `if` block. Also, the underlying components may respond faster if the key isn't available… but at least we can try not making the situation worse!
		// Also, this takes some time as we're allocating memory, but in the case of err==nil the operation there takes some time too.
		fileKeyBytes = make([]byte, 32)
	} else {
		err = fileKeyJWK.Raw(&fileKeyBytes)
		if err != nil {
			fileKeyBytes = make([]byte, 32)
		}
	}

	// Import the file key
	fk, err := importFileKey(fileKeyBytes, manifestObj.NoncePrefix, manifestObj.Cipher)
	if err != nil {
		return nil, err
	}

	// Now validate the MAC of the header
	err = fk.VerifyHeaderSignature(manifest, mac)
	if err != nil {
		return nil, err
	}

	// Start a background goroutine to perform the encryption, and return the stream to the caller
	// From now on, errors are returned as errors on the stream
	outR, outW := io.Pipe()
	go processSegments(fk, in, outW, fk.DecryptSegment, SegmentSize+SegmentOverhead)

	return outR, nil
}

// Reads all segment from the input stream, either plaintext or ciphertext, and process them (encrypt or decrypt them)
func processSegments(fk fileKey, in io.Reader, out *io.PipeWriter, processFn processSegmentFn, segmentSize int) {
	// Get a buffer from the pool
	buf := BufPool.Get().(*[]byte)
	defer func() {
		BufPool.Put(buf)
	}()

	// Read from the input stream till the end, one segment at a time
	var (
		err          error
		segment      uint32
		done         bool
		hasCarryover bool
		carryover    byte
		n, nn        int
	)
	for !done {
		n = 0

		// Add the carryover byte if we have one
		if hasCarryover {
			(*buf)[0] = carryover
			n = 1
			hasCarryover = false
		}

		// Read a segment from the buffer till we have a full segment + 1 byte or an error (could be EOF).
		// We are reading an extra byte because we need to understand if we've reached the end of the file.
		// Otherwise, if the input stream's data were exactly multiples of segmentSize, we wouldn't have a way to know.
		// Note that the underlying buffer may be larger, so we may not fill it up ever, and that's ok (i.e. if segmentSize == SegmentSize, we are reading an extra 1 byte rather than 17)
		for n < (segmentSize+1) && err == nil {
			nn, err = in.Read((*buf)[n:(segmentSize + 1)])
			n += nn
		}

		// Ignore EOF errors, which mean that the input stream is done
		// We will still need to continue processing whatever data we have
		if err != nil && !errors.Is(err, io.EOF) {
			// In case of any other error, close the out stream with the error
			_ = out.CloseWithError(err)
			return
		}

		// If we read an extra byte, set that as carryover
		// Otherwise, this means that we have the last segment
		if n > segmentSize {
			carryover = (*buf)[n-1]
			hasCarryover = true
			n--
		} else {
			done = true
		}

		// It's ok if we got less than a full segment, as long as this was the last segment (i.e. the stream is done)
		// Realistically, this should never happen, because in this case we would have had an error returned by in.Read.
		if n < segmentSize && !done {
			_ = out.CloseWithError(io.ErrUnexpectedEOF)
			return
		}

		// A completely empty segment is ok only if this is the first segment (i.e. the input was empty)
		// Note that here, we've already checked and made sure that the input stream is done
		if n == 0 {
			if segment != 0 {
				// Realistically, it should be impossible for us to get to this point as well, as there would have been a carryover from the previous iteration.
				_ = out.CloseWithError(io.ErrUnexpectedEOF)
				return
			}
			break
		}

		// We can now process the segment
		err = processFn(out, (*buf)[:n], segment, done)
		if err != nil {
			_ = out.CloseWithError(fmt.Errorf("error processing segment %d: %w", segment, err))
			return
		}

		// Proceed to the next segment if not done
		if !done && segment == 1<<32-1 {
			// We're about to overflow
			_ = out.CloseWithError(errors.New("input stream is too large"))
			return
		}
		segment++
	}

	// Close the out stream as done
	_ = out.Close()
}

func readHeader(in *io.Reader) (manifest []byte, mac []byte, err error) {
	// Get a buffer from the pool
	buf := BufPool.Get().(*[]byte)
	defer func() {
		BufPool.Put(buf)
	}()

	// Read the first segment to get the header
	// We know that the header (including the MAC) aren't larger than a single segment
	// Keep reading from the buffer until we get at least 3 newline characters (or an error)
	var (
		n, nn, i, ul int
		newlines     int
		lastNewline  int
		line         []byte
	)
	for newlines < 3 && err == nil {
		// Even though the maximum size for the header is 1 segment (64KB + 16 bytes), read 512 bytes at a time at most, since most headers are much smaller than that
		ul = n + 512
		if ul > SegmentSize {
			ul = SegmentSize
		}
		if n == ul {
			break
		}
		nn, err = (*in).Read((*buf)[n:SegmentSize])
		if nn <= 0 {
			continue
		}

		for i = n; i < (n+nn) && newlines < 3; i++ {
			if (*buf)[i] != '\n' {
				continue
			}

			if i <= lastNewline {
				return nil, nil, errors.New("invalid format")
			}
			line = (*buf)[lastNewline:i]
			switch newlines {
			case 0:
				// First line must be the scheme name
				if string(line) != SchemeName {
					return nil, nil, errors.New("unsupported scheme")
				}
			case 1:
				// Second line is the manifest
				manifest = line
			case 2:
				// Third line is the MAC
				mac = line
			}
			newlines++
			lastNewline = i + 1
		}
		n += nn
	}

	// Ensure we have a manifest and MAC
	if newlines < 1 {
		return nil, nil, errors.New("scheme name not found")
	}
	if len(manifest) == 0 {
		return nil, nil, errors.New("manifest not found")
	}
	if len(mac) == 0 {
		return nil, nil, errors.New("message authentication code not found")
	}

	// Whatever data we read extra, add it back to the beginning of the stream
	if n > lastNewline {
		// We need to copy the data because the buffer will be given back
		extraBytes := make([]byte, n-lastNewline)
		copy(extraBytes, (*buf)[(lastNewline):n])
		*in = io.MultiReader(bytes.NewReader(extraBytes), *in)
	}

	return manifest, mac, nil
}

func writeOrClosePipe(w *io.PipeWriter, b []byte) bool {
	_, err := w.Write(b)
	if err != nil {
		_ = w.CloseWithError(fmt.Errorf("failed to write to the stream: %w", err))
		return false
	}
	return true
}
