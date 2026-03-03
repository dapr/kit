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
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// OnHUP returns a channel that yields a new context each time a SIGHUP signal
// is received. Each context is canceled when the next SIGHUP arrives or when
// the parent context is canceled. The channel is closed when the parent context
// is canceled.
func OnHUP(ctx context.Context) <-chan context.Context {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)

	ctxhupCh := make(chan context.Context, 1)

	go func() {
		defer signal.Stop(sigCh)
		defer close(ctxhupCh)

		for {
			ctxhup, cancel := context.WithCancelCause(ctx)

			// Ensure we don't block indefinitely on send if the consumer stops reading.
			select {
			case ctxhupCh <- ctxhup:
				// Sent successfully; proceed to wait for signal or parent cancellation.
			case <-ctx.Done():
				// Parent context canceled before we could send; clean up and exit.
				cancel(ctx.Err())
				return
			}

			select {
			case sig := <-sigCh:
				log.Infof(`Received signal '%s'; restarting`, sig)
				cancel(errors.New("received SIGHUP"))
			case <-ctx.Done():
				cancel(ctx.Err())
				return
			}
		}
	}()

	return ctxhupCh
}
