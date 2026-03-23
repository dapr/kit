//go:build !windows
// +build !windows

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

package signals

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// ReloadPipeName returns the named pipe path that would be used on Windows.
// On POSIX systems this is not used (SIGHUP is used instead), but is provided
// so that cross-platform code compiles.
func ReloadPipeName(int) string {
	return ""
}

// listenPipe is not used on POSIX systems (SIGHUP is used instead), but is
// provided for compilation completeness.
func listenPipe(string) (net.Listener, error) {
	return nil, fmt.Errorf("named pipe listener is not supported on this platform")
}

// SignalReload sends SIGHUP to the process with the given PID on POSIX
// systems, triggering a runtime reload.
func SignalReload(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}
	return proc.Signal(syscall.SIGHUP)
}
