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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptionRSAPKCS1v15(t *testing.T) {
	const message = "❤️ 45.5096759, 11.4108900"

	key, err := ParseKey([]byte(privateKeyRSAPKCS8), "application/x-pem-file")
	require.NoError(t, err)
	require.NotNil(t, key)

	var ciphertext []byte
	t.Run("encrypt", func(t *testing.T) {
		ciphertext, err = EncryptPublicKey([]byte(message), Algorithm_RSA1_5, key, nil)
		require.NoError(t, err)
		require.NotNil(t, ciphertext)
	})

	t.Run("decrypt", func(t *testing.T) {
		plaintext, err := DecryptPrivateKey(ciphertext, Algorithm_RSA1_5, key, nil)
		require.NoError(t, err)
		require.NotNil(t, plaintext)
		require.Equal(t, message, string(plaintext))
	})
}

func TestEncryptionRSAOAEP(t *testing.T) {
	const message = "❤️ 45.5096759, 11.4108900"

	key, err := ParseKey([]byte(privateKeyRSAPKCS8), "application/x-pem-file")
	require.NoError(t, err)
	require.NotNil(t, key)

	algs := []string{
		Algorithm_RSA_OAEP,
		Algorithm_RSA_OAEP_256,
		Algorithm_RSA_OAEP_384,
		Algorithm_RSA_OAEP_512,
	}

	for _, alg := range algs {
		t.Run(alg, func(t *testing.T) {
			var ciphertext []byte
			t.Run("encrypt", func(t *testing.T) {
				ciphertext, err = EncryptPublicKey([]byte(message), alg, key, nil)
				require.NoError(t, err)
				require.NotNil(t, ciphertext)
			})

			t.Run("decrypt", func(t *testing.T) {
				plaintext, err := DecryptPrivateKey(ciphertext, alg, key, nil)
				require.NoError(t, err)
				require.NotNil(t, plaintext)
				require.Equal(t, message, string(plaintext))
			})
		})
	}
}
