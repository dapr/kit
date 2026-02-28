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

// Note this file is not built on Windows, as we depend on syscall methods not available on Windows.

import (
	"context"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext(t *testing.T) {
	signal.Reset()

	t.Run("SIGINT should cancel context", func(t *testing.T) {
		defer signal.Reset()
		onlyOneSignalHandler = make(chan struct{})

		ctx := Context()
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
		select {
		case <-ctx.Done():
		case <-time.After(1 * time.Second):
			t.Error("context should be cancelled in time")
		}
	})

	t.Run("SIGTERM should cancel context", func(t *testing.T) {
		defer signal.Reset()
		onlyOneSignalHandler = make(chan struct{})

		ctx := Context()
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		select {
		case <-ctx.Done():
		case <-time.After(1 * time.Second):
			t.Error("context should be cancelled in time")
		}
	})

	t.Run("context cause should contain signal information", func(t *testing.T) {
		defer signal.Reset()
		onlyOneSignalHandler = make(chan struct{})

		ctx := Context()
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
		select {
		case <-ctx.Done():
			cause := context.Cause(ctx)
			require.Error(t, cause)
			assert.Contains(t, cause.Error(), "interrupt",
				"cause should contain signal name, got: %s", cause.Error())
		case <-time.After(1 * time.Second):
			t.Error("context should be cancelled in time")
		}
	})
}

func TestContextWithHUP(t *testing.T) {
	signal.Reset()

	t.Run("SIGHUP should cancel context", func(t *testing.T) {
		defer signal.Reset()

		ctx := <-ContextWithHUP(t.Context())
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGHUP))
		select {
		case <-ctx.Done():
		case <-time.After(1 * time.Second):
			t.Error("context should be cancelled in time")
		}
	})

	t.Run("parent context cancellation should cancel derived context", func(t *testing.T) {
		defer signal.Reset()

		parent, cancel := context.WithCancel(t.Context())
		ctx := <-ContextWithHUP(parent)

		cancel()

		select {
		case <-ctx.Done():
		case <-time.After(1 * time.Second):
			t.Error("context should be cancelled in time")
		}
	})

	t.Run("multiple HUP contexts can be created", func(t *testing.T) {
		defer signal.Reset()

		ctx1 := <-ContextWithHUP(t.Context())
		ctx2 := <-ContextWithHUP(t.Context())

		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGHUP))

		select {
		case <-ctx1.Done():
		case <-time.After(1 * time.Second):
			t.Error("ctx1 should be cancelled in time")
		}

		select {
		case <-ctx2.Done():
		case <-time.After(1 * time.Second):
			t.Error("ctx2 should be cancelled in time")
		}
	})
}
