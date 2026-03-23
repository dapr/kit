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

package pem

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
)

func TestEncodePrivateKey(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate Ed25519 key: %v", err)
	}

	tests := []struct {
		name      string
		key       any
		wantErr   bool
		errSubstr string
	}{
		{
			name: "ECDSA P-256",
			key:  ecKey,
		},
		{
			name: "RSA 2048",
			key:  rsaKey,
		},
		{
			name: "Ed25519",
			key:  ed25519Key,
		},
		{
			name:      "unsupported type",
			key:       "not a key",
			wantErr:   true,
			errSubstr: "unsupported key type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodePrivateKey(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(encoded) == 0 {
				t.Fatal("encoded output is empty")
			}

			decoded, err := DecodePEMPrivateKey(encoded)
			if err != nil {
				t.Fatalf("roundtrip decode failed: %v", err)
			}

			if !keysEqual(t, tt.key, decoded) {
				t.Fatal("roundtrip key does not match original")
			}
		})
	}
}

// keysEqual compares the original key with the decoded signer using
// each key type's Equal method.
func keysEqual(t *testing.T, original any, decoded crypto.Signer) bool {
	t.Helper()

	switch orig := original.(type) {
	case *ecdsa.PrivateKey:
		d, ok := decoded.(*ecdsa.PrivateKey)
		if !ok {
			t.Errorf("decoded key type %T, want *ecdsa.PrivateKey", decoded)
			return false
		}

		return orig.Equal(d)
	case *rsa.PrivateKey:
		d, ok := decoded.(*rsa.PrivateKey)
		if !ok {
			t.Errorf("decoded key type %T, want *rsa.PrivateKey", decoded)
			return false
		}

		return orig.Equal(d)
	case ed25519.PrivateKey:
		d, ok := decoded.(ed25519.PrivateKey)
		if !ok {
			t.Errorf("decoded key type %T, want ed25519.PrivateKey", decoded)
			return false
		}

		return orig.Equal(d)
	default:
		t.Errorf("unknown key type %T", original)
		return false
	}
}
