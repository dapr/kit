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
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

// Long message, which we will hash in the init method
const message = "Nel mezzo del cammin di nostra vita\nmi ritrovai per una selva oscura,\nché la diritta via era smarrita.\n\nAhi quanto a dir qual era è cosa dura\nesta selva selvaggia e aspra e forte\nche nel pensier rinova la paura!\n\nTant' è amara che poco è più morte;\nma per trattar del ben ch'i' vi trovai,\ndirò de l'altre cose ch'i' v'ho scorte.\n\nIo non so ben ridir com' i' v'intrai,\ntant' era pien di sonno a quel punto\nche la verace via abbandonai.\n\nMa poi ch'i' fui al piè d'un colle giunto,\nlà dove terminava quella valle\nche m'avea di paura il cor compunto,\n\nguardai in alto e vidi le sue spalle\nvestite già de' raggi del pianeta\nche mena dritto altrui per ogne calle."

var messageHash []byte

func init() {
	h := sha256.Sum256([]byte(message))
	messageHash = h[:]
}

func TestSigningRSAPKCS1v15(t *testing.T) {
	key, err := ParseKey([]byte(privateKeyRSAPKCS8), "application/x-pem-file")
	require.NoError(t, err)
	require.NotNil(t, key)

	var signature []byte
	t.Run("sign", func(t *testing.T) {
		signature, err = SignPrivateKey(messageHash, Algorithm_RS256, key)
		require.NoError(t, err)
		require.NotNil(t, signature)
	})

	t.Run("verify", func(t *testing.T) {
		var valid bool
		valid, err = VerifyPublicKey(messageHash, signature, Algorithm_RS256, key)
		require.NoError(t, err)
		require.True(t, valid)
	})
}

func TestSigningRSAPSS(t *testing.T) {
	key, err := ParseKey([]byte(privateKeyRSAPKCS8), "application/x-pem-file")
	require.NoError(t, err)
	require.NotNil(t, key)

	var signature []byte
	t.Run("sign", func(t *testing.T) {
		signature, err = SignPrivateKey(messageHash, Algorithm_PS256, key)
		require.NoError(t, err)
		require.NotNil(t, signature)
	})

	t.Run("verify", func(t *testing.T) {
		var valid bool
		valid, err = VerifyPublicKey(messageHash, signature, Algorithm_PS256, key)
		require.NoError(t, err)
		require.True(t, valid)
	})
}

func TestSigningECDSA(t *testing.T) {
	key, err := ParseKey([]byte(privateKeyP256PKCS8), "application/x-pem-file")
	require.NoError(t, err)
	require.NotNil(t, key)

	var signature []byte
	t.Run("sign", func(t *testing.T) {
		signature, err = SignPrivateKey(messageHash, Algorithm_ES256, key)
		require.NoError(t, err)
		require.NotNil(t, signature)
	})

	t.Run("verify", func(t *testing.T) {
		var valid bool
		valid, err = VerifyPublicKey(messageHash, signature, Algorithm_ES256, key)
		require.NoError(t, err)
		require.True(t, valid)
	})
}

func TestSigningEdDSA(t *testing.T) {
	// When using EdDSA, we pass the actual mesage and not the hash
	key, err := ParseKey([]byte(privateKeyEd25519JSON), "application/json")
	require.NoError(t, err)
	require.NotNil(t, key)

	var signature []byte
	t.Run("sign", func(t *testing.T) {
		signature, err = SignPrivateKey([]byte(message), Algorithm_EdDSA, key)
		require.NoError(t, err)
		require.NotNil(t, signature)
	})

	t.Run("verify", func(t *testing.T) {
		var valid bool
		valid, err = VerifyPublicKey([]byte(message), signature, Algorithm_EdDSA, key)
		require.NoError(t, err)
		require.True(t, valid)
	})
}
