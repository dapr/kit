/*
Copyright 2025 The Dapr Authors
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

package ctesting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/concurrency/ctesting/internal"
)

type RunnerFn func(context.Context, assert.TestingT)

// Assert runs the provided test functions in parallel and asserts that they
// all pass.
func Assert(t *testing.T, runners ...RunnerFn) {
	t.Helper()

	if len(runners) == 0 {
		require.Fail(t, "at least one runner function is required")
	}

	tt := internal.Assert(t)

	ctx, cancel := context.WithCancelCause(t.Context())
	t.Cleanup(func() { cancel(nil) })

	doneCh := make(chan struct{}, len(runners))
	for _, runner := range runners {
		go func(rfn RunnerFn) {
			rfn(ctx, tt)
			if errs := tt.Errors(); len(errs) > 0 {
				cancel(errors.Join(errs...))
			}
			doneCh <- struct{}{}
		}(runner)
	}

	for range len(runners) {
		select {
		case <-doneCh:
		case <-t.Context().Done():
			require.FailNow(t, "test context was cancelled before all runners completed")
		}
	}

	for _, err := range tt.Errors() {
		assert.NoError(t, err)
	}
}

// AssertCleanup runs the provided test functions in parallel and asserts that they
// all pass, only after Cleanup,.
func AssertCleanup(t *testing.T, runners ...concurrency.Runner) {
	t.Helper()

	ctx, cancel := context.WithCancelCause(t.Context())

	errCh := make(chan error, len(runners))
	for _, runner := range runners {
		go func(rfn concurrency.Runner) {
			errCh <- rfn(ctx)
		}(runner)
	}

	t.Cleanup(func() {
		cancel(nil)
		for range runners {
			select {
			case err := <-errCh:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				assert.Fail(t, "timeout waiting for runner to stop")
			}
		}
	})
}
