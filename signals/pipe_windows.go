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
	"fmt"
	"net"
	"strconv"
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
// The pipe is secured so that the creating user (Creator Owner),
// Built-in Administrators, and Local System have full access.
func listenPipe(name string) (net.Listener, error) {
	return winio.ListenPipe(name, &winio.PipeConfig{
		// CO = Creator Owner, BA = Built-in Administrators, SY = Local System.
		SecurityDescriptor: "D:P(A;;GA;;;CO)(A;;GA;;;BA)(A;;GA;;;SY)",
	})
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
