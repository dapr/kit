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

func TestKeyAlgorithmValidate(t *testing.T) {
	tests := []struct {
		name    string
		a       KeyAlgorithm
		want    KeyAlgorithm
		wantErr bool
	}{
		{name: string(KeyAlgorithmAESKW), a: KeyAlgorithmAESKW, want: KeyAlgorithmAESKW},
		{name: string(KeyAlgorithmAES) + " alias", a: KeyAlgorithmAES, want: KeyAlgorithmAESKW},
		{name: string(KeyAlgorithmRSAOAEP256), a: KeyAlgorithmRSAOAEP256, want: KeyAlgorithmRSAOAEP256},
		{name: string(KeyAlgorithmRSA) + " alias", a: KeyAlgorithmRSA, want: KeyAlgorithmRSAOAEP256},
		{name: "invalid algorithm", a: "foo", wantErr: true},
		{name: "empty algorithm", a: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("KeyAlgorithm.Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			} else if err != nil {
				t.Errorf("KeyAlgorithm.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("KeyAlgorithm.Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyAlgorithmMarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		a       KeyAlgorithm
		want    string
		wantErr bool
	}{
		{name: string(KeyAlgorithmAESKW), a: KeyAlgorithmAESKW, want: "1"},
		{name: string(KeyAlgorithmAES) + " alias", a: KeyAlgorithmAES, want: "1"},
		{name: string(KeyAlgorithmRSAOAEP256), a: KeyAlgorithmRSAOAEP256, want: "2"},
		{name: string(KeyAlgorithmRSA) + " alias", a: KeyAlgorithmRSA, want: "2"},
		{name: "invalid algorithm", a: "foo", want: "0"},
		{name: "empty algorithm", a: "", want: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.a)
			if (err != nil) != tt.wantErr {
				t.Errorf("KeyAlgorithm.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("KeyAlgorithm.MarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyAlgorithmUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    KeyAlgorithm
		wantErr bool
	}{
		{name: string(KeyAlgorithmAESKW), message: "1", want: KeyAlgorithmAESKW},
		{name: string(KeyAlgorithmRSAOAEP256), message: "2", want: KeyAlgorithmRSAOAEP256},
		{name: "invalid ID", message: "99", wantErr: true},
		{name: "empty", message: "", wantErr: true},
		{name: "JSON null", message: "null", wantErr: true},
		{name: "JSON string", message: `"AES"`, wantErr: true},
		{name: "JSON object", message: `{"foo":1}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a KeyAlgorithm
			err := json.Unmarshal([]byte(tt.message), &a)
			if (err != nil) != tt.wantErr {
				t.Errorf("KeyAlgorithm.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if a != tt.want {
				t.Errorf("KeyAlgorithm.UnmarshalJSON() = %v, want %v", a, tt.want)
			}
		})
	}
}
