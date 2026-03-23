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
	"context"
	"errors"
	"fmt"
	"os"
)

var shutdownSignals = []os.Signal{os.Interrupt}

// OnHUP returns a channel that yields a new context each time a reload signal
// is received via a Windows named pipe. Each context is canceled when the next
// reload signal arrives or when the parent context is canceled. The channel is
// closed when the parent context is canceled.
//
// On Windows, SIGHUP is not supported. Instead, this function listens on a
// named pipe (\\.\pipe\dapr-reload-<PID>). Any connection to the pipe
// triggers a reload, equivalent to sending SIGHUP on POSIX systems.
func OnHUP(ctx context.Context) <-chan context.Context {
	ctxhupCh := make(chan context.Context, 1)

	go func() {
		defer close(ctxhupCh)

		pipeName := ReloadPipeName(os.Getpid())
		listener, err := listenPipe(pipeName)
		if err != nil {
			log.Errorf("Failed to create reload named pipe %s: %v", pipeName, err)
			// Fall back to the old no-op behavior: send ctx once, wait for
			// cancellation.
			ctxhupCh <- ctx
			<-ctx.Done()
			return
		}

		log.Infof("Listening for reload signals on named pipe %s", pipeName)

		go func() {
			<-ctx.Done()
			listener.Close()
		}()

		for {
			ctxhup, cancel := context.WithCancelCause(ctx)

			select {
			case ctxhupCh <- ctxhup:
			case <-ctx.Done():
				cancel(ctx.Err())
				return
			}

			// Wait for a connection on the named pipe. A connection (and
			// immediate close) is the reload trigger, equivalent to SIGHUP.
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					cancel(ctx.Err())
					return
				}
				log.Warnf("Error accepting reload pipe connection: %v", err)
				cancel(fmt.Errorf("pipe accept error: %w", err))
				continue
			}
			conn.Close()

			log.Info("Received reload signal via named pipe; restarting")
			cancel(errors.New("received reload signal via named pipe"))
		}
	}()

	return ctxhupCh
}
