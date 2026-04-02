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
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnHUP_WindowsPipe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctxCh := OnHUP(ctx)

	// First context should be emitted immediately.
	var firstCtx context.Context
	select {
	case c, ok := <-ctxCh:
		require.True(t, ok, "channel should yield a context")
		firstCtx = c
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for initial context from OnHUP")
	}

	// First context should not be canceled yet.
	assert.NoError(t, firstCtx.Err(), "initial context should not be canceled")

	// Send a reload signal via the named pipe.
	err := SignalReload(os.Getpid())
	require.NoError(t, err, "SignalReload should connect to the pipe")

	// First context should be canceled after the reload signal.
	select {
	case <-firstCtx.Done():
		// expected
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first context to be canceled after reload signal")
	}

	// A new context should be emitted for the next reload cycle.
	var secondCtx context.Context
	select {
	case c, ok := <-ctxCh:
		require.True(t, ok, "channel should yield a second context")
		secondCtx = c
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second context from OnHUP")
	}

	assert.NoError(t, secondCtx.Err(), "second context should not be canceled yet")

	// Cancel the parent context to shut down the pipe listener.
	cancel()

	select {
	case <-secondCtx.Done():
		// expected
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second context to be canceled after parent cancel")
	}

	// Channel should be closed after parent context cancellation.
	select {
	case _, ok := <-ctxCh:
		assert.False(t, ok, "channel should be closed after parent context is canceled")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for channel to close")
	}
}

func TestReloadPipeName(t *testing.T) {
	name := ReloadPipeName(12345)
	assert.Equal(t, `\\.\pipe\dapr-reload-12345`, name)
}
