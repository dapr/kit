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
	"errors"
	"fmt"
	"net"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

// ReloadPipeName returns the named pipe path used by a dapr process with the
// given PID to listen for reload signals on Windows. This is exported so that
// external tooling (CLI, tests) can connect to trigger a reload.
func ReloadPipeName(pid int) string {
	return `\\.\pipe\dapr-reload-` + strconv.Itoa(pid)
}

// listenPipe creates a Windows named pipe listener at the given path.
// The pipe is secured so that the current user, Built-in Administrators,
// and Local System have full access.
//
// This now uses the user's actual SID rather than a Creator Owner (CO) SID.
func listenPipe(name string) (net.Listener, error) {
	sd, err := buildPipeSecurityDescriptor()
	if err != nil {
		return nil, fmt.Errorf("failed to build pipe security descriptor: %w", err)
	}
	return winio.ListenPipe(name, &winio.PipeConfig{
		SecurityDescriptor: sd,
	})
}

// buildPipeSecurityDescriptor constructs an SDDL string granting full access to
// the current user, Built-in Administrators, and Local System.
// Note: inheritance is disabled.
func buildPipeSecurityDescriptor() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to look up current user: %w", err)
	}
	if err := validateWindowsSID(u.Uid); err != nil {
		return "", fmt.Errorf("invalid current user SID %q: %w", u.Uid, err)
	}
	// Current User / Administrators / Local System
	return fmt.Sprintf("D:P(A;;GA;;;%s)(A;;GA;;;BA)(A;;GA;;;SY)", u.Uid), nil
}

// validateWindowsSID returns a non-nil error if s is not a syntactically valid
func validateWindowsSID(s string) error {
	if s == "" {
		return errors.New("empty SID")
	}
	if !strings.HasPrefix(s, "S-1-") {
		return errors.New("not a Windows SID")
	}
	for _, part := range strings.Split(s[2:], "-") {
		if part == "" {
			return errors.New("malformed SID")
		}
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return fmt.Errorf("malformed SID component %q", part)
		}
	}
	return nil
}

// SignalReload connects to the reload named pipe for the given PID, triggering
// a reload of that dapr process.
func SignalReload(pid int) error {
	pipeName := ReloadPipeName(pid)
	timeout := 5 * time.Second
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to reload pipe %s: %w", pipeName, err)
	}
	conn.Close()
	return nil
}
