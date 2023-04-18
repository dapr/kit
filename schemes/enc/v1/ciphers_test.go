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
	"encoding/json"
	"testing"
)

func TestCipherValidate(t *testing.T) {
	tests := []struct {
		name    string
		a       Cipher
		want    Cipher
		wantErr bool
	}{
		{name: string(CipherAESGCM), a: CipherAESGCM, want: CipherAESGCM},
		{name: string(CipherChaCha20Poly1305), a: CipherChaCha20Poly1305, want: CipherChaCha20Poly1305},
		{name: "invalid cipher", a: "foo", wantErr: true},
		{name: "empty cipher", a: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Cipher.Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			} else if err != nil {
				t.Errorf("Cipher.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Cipher.Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCipherMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		a       Cipher
		want    string
		wantErr bool
	}{
		{name: string(CipherAESGCM), a: CipherAESGCM, want: "1"},
		{name: string(CipherChaCha20Poly1305), a: CipherChaCha20Poly1305, want: "2"},
		{name: "invalid cipher", a: "foo", want: "0"},
		{name: "empty cipher", a: "", want: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.a)
			if (err != nil) != tt.wantErr {
				t.Errorf("Cipher.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Cipher.MarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCipherUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    Cipher
		wantErr bool
	}{
		{name: string(CipherAESGCM), message: "1", want: CipherAESGCM},
		{name: string(CipherChaCha20Poly1305), message: "2", want: CipherChaCha20Poly1305},
		{name: "invalid ID", message: "99", wantErr: true},
		{name: "empty", message: "", wantErr: true},
		{name: "JSON null", message: "null", wantErr: true},
		{name: "JSON string", message: `"AES"`, wantErr: true},
		{name: "JSON object", message: `{"foo":1}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Cipher
			err := json.Unmarshal([]byte(tt.message), &a)
			if (err != nil) != tt.wantErr {
				t.Errorf("Cipher.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if a != tt.want {
				t.Errorf("Cipher.UnmarshalJSON() = %v, want %v", a, tt.want)
			}
		})
	}
}
