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
	"os"
	"os/signal"
	"syscall"
)

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// ContextWithHUP returns a context which will be canceled when the SIGHUP
// signal is caught. The returned context will also be canceled when the parent
// context is canceled.
func ContextWithHUP(ctx context.Context) context.Context {
	ctxhup, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)

	go func() {
		defer signal.Stop(sigCh)
		select {
		case sig := <-sigCh:
			log.Infof(`Received signal '%s'; restarting`, sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctxhup
}
