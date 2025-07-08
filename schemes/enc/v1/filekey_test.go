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
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileKey(t *testing.T) {
	t.Run("getCipher", func(t *testing.T) {
		// We need to set a payloadKey for this test, even if empty
		payloadKey := make([]byte, 32)

		tests := []struct {
			name    string
			cipher  Cipher
			wantErr bool
		}{
			{name: string(CipherAESGCM), cipher: CipherAESGCM, wantErr: false},
			{name: string(CipherChaCha20Poly1305), cipher: CipherChaCha20Poly1305, wantErr: false},
			{name: "invalid cipher", cipher: Cipher("invalid"), wantErr: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				k := fileKey{
					cipher:     tt.cipher,
					payloadKey: payloadKey,
				}
				gotAead, err := k.getCipher()
				if (err != nil) != tt.wantErr {
					t.Errorf("fileKey.getCipher() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if err == nil && gotAead == nil {
					t.Error("fileKey.getCipher() = nil")
				}
			})
		}

		t.Run("with invalid AES-GCM key", func(t *testing.T) {
			k := fileKey{
				cipher:     CipherAESGCM,
				payloadKey: make([]byte, 10),
			}
			_, err := k.getCipher()
			require.Error(t, err)
			require.ErrorContains(t, err, "crypto/aes: invalid key size 10")
		})

		t.Run("with invalid ChaCha20-Poly1305 key", func(t *testing.T) {
			k := fileKey{
				cipher:     CipherChaCha20Poly1305,
				payloadKey: make([]byte, 10),
			}
			_, err := k.getCipher()
			require.Error(t, err)
			require.ErrorContains(t, err, "chacha20poly1305: bad key length")
		})
	})

	t.Run("nonceForSegment", func(t *testing.T) {
		noncePrefix := []byte{1, 2, 3, 4, 5, 6, 7}
		type args struct {
			num  uint32
			last bool
		}
		tests := []struct {
			name string
			args args
			want []byte
		}{
			{name: "segment 0", args: args{num: 0, last: false}, want: []byte{1, 2, 3, 4, 5, 6, 7, 0, 0, 0, 0, 0}},
			{name: "segment 0 is last", args: args{num: 0, last: true}, want: []byte{1, 2, 3, 4, 5, 6, 7, 0, 0, 0, 0, 1}},
			{name: "segment 1", args: args{num: 1, last: false}, want: []byte{1, 2, 3, 4, 5, 6, 7, 0, 0, 0, 1, 0}},
			{name: "segment 1 is last", args: args{num: 1, last: true}, want: []byte{1, 2, 3, 4, 5, 6, 7, 0, 0, 0, 1, 1}},
			{name: "segment 2000 is last", args: args{num: 2_000, last: true}, want: []byte{1, 2, 3, 4, 5, 6, 7, 0, 0, 0x7, 0xD0, 1}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				k := fileKey{
					noncePrefix: noncePrefix,
				}
				if got := k.nonceForSegment(tt.args.num, tt.args.last); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("fileKey.nonceForSegment() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("deriveKey", func(t *testing.T) {
		// We are testing importFileKey to validate the behavior of deriveKey primarily
		key := mustDecodeHexString("4ae3be77186824592c9b6aa625f6ac1ba16fddf60359f3342e6761883a1f82d4")
		noncePrefix := []byte{1, 2, 3, 4, 5, 6, 7}
		expectHeaderKey := mustDecodeHexString("f702256c0abd7ed84ebee9f897abfbf8e5ad4d5fc19e7907e3e6844be1a24189")
		expectPayloadKey := mustDecodeHexString("8cbcf6b6b426c7291e8e1d9f0d8989a20fe93cb38ca8513db1a2b08fa2ecb883")

		fk, err := importFileKey(key, noncePrefix, CipherAESGCM)
		require.NoError(t, err)
		require.Equal(t, expectHeaderKey, fk.headerKey)
		require.Equal(t, expectPayloadKey, fk.payloadKey)
	})

	t.Run("headerMessage", func(t *testing.T) {
		// Validate that headerMessage returns the right message, and that there's a newline at the end
		const manifest = `{"foo":"bar"}`
		const expect = SchemeName + "\n" + manifest + "\n"
		t.Log(hex.EncodeToString([]byte(expect)))

		got := fileKey{}.headerMessage([]byte(manifest))
		require.Equal(t, expect, string(got))
	})

	t.Run("computeHeaderSignature", func(t *testing.T) {
		const manifest = `{"foo":"bar"}`
		key := mustDecodeHexString("4ae3be77186824592c9b6aa625f6ac1ba16fddf60359f3342e6761883a1f82d4")
		noncePrefix := []byte{1, 2, 3, 4, 5, 6, 7}
		expectSignature := mustDecodeHexString("ac54af18f2cf36631ec41af34dcdd32e526a26df6975a3a6a83c78d997ded017")

		fk, err := importFileKey(key, noncePrefix, CipherAESGCM)
		require.NoError(t, err)

		msg := fk.headerMessage([]byte(manifest))
		sig, err := fk.computeHeaderSignature(msg)
		require.NoError(t, err)
		require.Equal(t, expectSignature, sig)
	})
}

func mustDecodeHexString(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
