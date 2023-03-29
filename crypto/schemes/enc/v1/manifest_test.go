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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		wantErr  string
	}{
		{
			name: "all properties included",
			manifest: &Manifest{
				KeyName:              "mykey",
				KeyWrappingAlgorithm: KeyAlgorithmAESKW,
				WFK:                  []byte{0x01, 0x02, 0x03},
				Cipher:               CipherAESGCM,
				NoncePrefix:          []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
		},
		{
			name: "key name is optional",
			manifest: &Manifest{
				KeyWrappingAlgorithm: KeyAlgorithmAESKW,
				WFK:                  []byte{0x01, 0x02, 0x03},
				Cipher:               CipherAESGCM,
				NoncePrefix:          []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
		},
		{
			name: "missing key wrapping algorithm",
			manifest: &Manifest{
				WFK:         []byte{0x01, 0x02, 0x03},
				Cipher:      CipherAESGCM,
				NoncePrefix: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
			wantErr: "key wrapping algorithm is invalid",
		},
		{
			name: "missing wrapped file key",
			manifest: &Manifest{
				KeyWrappingAlgorithm: KeyAlgorithmAESKW,
				Cipher:               CipherAESGCM,
				NoncePrefix:          []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
			wantErr: "wrapped file key is empty",
		},
		{
			name: "missing cipher",
			manifest: &Manifest{
				KeyWrappingAlgorithm: KeyAlgorithmAESKW,
				WFK:                  []byte{0x01, 0x02, 0x03},
				NoncePrefix:          []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
			wantErr: "cipher is invalid",
		},
		{
			name: "missing nonce prefix",
			manifest: &Manifest{
				KeyWrappingAlgorithm: KeyAlgorithmAESKW,
				WFK:                  []byte{0x01, 0x02, 0x03},
				Cipher:               CipherAESGCM,
			},
			wantErr: "nonce prefix is invalid",
		},
		{
			name: "nonce prefix too short",
			manifest: &Manifest{
				KeyWrappingAlgorithm: KeyAlgorithmAESKW,
				WFK:                  []byte{0x01, 0x02, 0x03},
				Cipher:               CipherAESGCM,
				NoncePrefix:          []byte{0x01, 0x02, 0x03, 0x04},
			},
			wantErr: "nonce prefix is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr == "" && err != nil ||
				tt.wantErr != "" && err == nil ||
				tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Manifest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManifestJSON(t *testing.T) {
	testFn := func(keyName string, expectEnc string) func(t *testing.T) {
		return func(t *testing.T) {
			m := &Manifest{
				KeyName:              keyName,
				KeyWrappingAlgorithm: KeyAlgorithmAESKW,
				WFK:                  []byte{0x01, 0x02, 0x03},
				Cipher:               CipherAESGCM,
				NoncePrefix:          []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			}

			// Marshal
			enc, err := json.Marshal(m)
			require.NoError(t, err)

			// Compact the JSON before checking for equality
			encBuf := &bytes.Buffer{}
			err = json.Compact(encBuf, enc)
			require.NoError(t, err)

			require.Equal(t, expectEnc, encBuf.String())

			// Unmarshal
			var dec Manifest
			err = json.Unmarshal(enc, &dec)
			require.NoError(t, err)
			assert.Equal(t, m.KeyWrappingAlgorithm, dec.KeyWrappingAlgorithm)
			assert.Equal(t, m.WFK, dec.WFK)
			assert.Equal(t, m.Cipher, dec.Cipher)
			assert.Equal(t, m.NoncePrefix, dec.NoncePrefix)
		}
	}

	t.Run("without key name", testFn("", `{"kw":1,"wfk":"AQID","cph":1,"np":"AQIDBAUGBw=="}`))
	t.Run("with key name", testFn("mykey", `{"k":"mykey","kw":1,"wfk":"AQID","cph":1,"np":"AQIDBAUGBw=="}`))
}
