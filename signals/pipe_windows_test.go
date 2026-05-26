/*
Copyright 2026 The Dapr Authors
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

package signals

import (
	"os/user"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWindowsSID(t *testing.T) {
	tests := []struct {
		name    string
		sid     string
		wantErr bool
	}{
		{"valid local system", "S-1-5-18", false},
		{"valid user sid", "S-1-2-34-5678901234-5678901234-5678901234-5678901234", false},
		{"valid creator owner", "S-1-3-0", false},
		{"empty", "", true},
		{"missing prefix", "1-5-18", true},
		{"wrong revision", "S-2-5-18", true},
		{"trailing dash", "S-1-5-18-", true},
		{"double dash", "S-1-5--18", true},
		{"non-numeric component", "S-1-5-abc", true},
		{"negative component", "S-1-5--1", true},
		{"overflow component", "S-1-5-18446744073709551616", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWindowsSID(tc.sid)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBuildPipeSecurityDescriptor(t *testing.T) {
	sd, err := buildPipeSecurityDescriptor()
	require.NoError(t, err)

	u, err := user.Current()
	require.NoError(t, err)

	// Must be a protected DACL (D:P) granting GA to current user, built-in admins and system.
	assert.True(t, strings.HasPrefix(sd, "D:P"), "SDDL should start with protected DACL marker D:P, got %q", sd)
	assert.Contains(t, sd, "(A;;GA;;;"+u.Uid+")", "SDDL should grant GA to current user SID")
	assert.Contains(t, sd, "(A;;GA;;;BA)", "SDDL should grant GA to Built-in Administrators")
	assert.Contains(t, sd, "(A;;GA;;;SY)", "SDDL should grant GA to Local System")
}
