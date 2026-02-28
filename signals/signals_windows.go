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
)

var shutdownSignals = []os.Signal{os.Interrupt}

// OnHUP is a no-op on Windows as SIGHUP is not supported. It returns a channel
// that yields a context derived from the parent, and closes when the parent
// context is canceled.
func OnHUP(ctx context.Context) <-chan context.Context {
	ctxCh := make(chan context.Context, 1)

	go func() {
		defer close(ctxCh)
		ctxCh <- ctx
		<-ctx.Done()
	}()

	return ctxCh
}
