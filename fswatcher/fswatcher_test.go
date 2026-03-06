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

package fswatcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSWatcher(t *testing.T) {
	runWatcher := func(t *testing.T, opts Options) <-chan struct{} {
		t.Helper()

		f, err := New(opts)
		require.NoError(t, err)

		errCh := make(chan error)
		ctx, cancel := context.WithCancel(t.Context())
		eventsCh := make(chan struct{})
		go func() {
			errCh <- f.Run(ctx, eventsCh)
		}()

		t.Cleanup(func() {
			cancel()
			select {
			case err := <-errCh:
				require.NoError(t, err)
			case <-time.After(time.Second):
				assert.Fail(t, "timeout waiting for watcher to stop")
			}
		})

		assert.Eventually(t, f.running.Load, time.Second, time.Millisecond*10)
		return eventsCh
	}

	t.Run("creating fswatcher with no directory should not error", func(t *testing.T) {
		runWatcher(t, Options{})
	})

	t.Run("running Run twice should error", func(t *testing.T) {
		fs, err := New(Options{})
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.NoError(t, fs.Run(ctx, make(chan struct{})))
		require.Error(t, fs.Run(ctx, make(chan struct{})))
	})

	t.Run("creating fswatcher with non-existent directory should error", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.RemoveAll(dir))
		_, err := New(Options{
			Targets: []string{dir},
		})
		require.Error(t, err)
	})

	t.Run("should fire event when event occurs on target file", func(t *testing.T) {
		fp := filepath.Join(t.TempDir(), "test.txt")
		require.NoError(t, os.WriteFile(fp, []byte{}, 0o600))
		eventsCh := runWatcher(t, Options{
			Targets: []string{fp},
		})
		assert.Empty(t, eventsCh)

		if runtime.GOOS == "windows" {
			// If running in windows, wait for notify to be ready.
			time.Sleep(time.Second)
		}
		require.NoError(t, os.WriteFile(fp, []byte{}, 0o600))

		select {
		case <-eventsCh:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout waiting for event")
		}
	})

	t.Run("should fire 2 events when event occurs on 2 file target", func(t *testing.T) {
		fp1 := filepath.Join(t.TempDir(), "test.txt")
		fp2 := filepath.Join(t.TempDir(), "test.txt")
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o600))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o600))
		eventsCh := runWatcher(t, Options{
			Targets: []string{fp1, fp2},
		})
		assert.Empty(t, eventsCh)
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o600))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o600))
		for range 2 {
			select {
			case <-eventsCh:
			case <-time.After(time.Second):
				assert.Fail(t, "timeout waiting for event")
			}
		}
	})

	t.Run("should fire 2 events when event occurs on 2 files inside target directory", func(t *testing.T) {
		dir := t.TempDir()
		fp1 := filepath.Join(dir, "test1.txt")
		fp2 := filepath.Join(dir, "test2.txt")
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o600))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o600))
		eventsCh := runWatcher(t, Options{
			Targets: []string{fp1, fp2},
		})
		if runtime.GOOS == "windows" {
			// If running in windows, wait for notify to be ready.
			time.Sleep(time.Second)
		}
		assert.Empty(t, eventsCh)
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o600))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o600))
		for range 2 {
			select {
			case <-eventsCh:
			case <-time.After(time.Second):
				assert.Fail(t, "timeout waiting for event")
			}
		}
	})

	t.Run("should fire 2 events when event occurs on 2 target directories", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		fp1 := filepath.Join(dir1, "test1.txt")
		fp2 := filepath.Join(dir2, "test2.txt")
		eventsCh := runWatcher(t, Options{
			Targets: []string{dir1, dir2},
		})
		assert.Empty(t, eventsCh)
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o600))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o600))
		for range 2 {
			select {
			case <-eventsCh:
			case <-time.After(time.Second):
				assert.Fail(t, "timeout waiting for event")
			}
		}
	})

	t.Run("should debounce burst of writes on same file", func(t *testing.T) {
		dir := t.TempDir()
		fp := filepath.Join(dir, "debounce.txt")

		// Create the file before starting the watcher so the initial creation
		// does not interfere with the debounce behavior we want to test.
		require.NoError(t, os.WriteFile(fp, []byte("initial"), 0o600))

		eventsCh := runWatcher(t, Options{
			Targets: []string{fp},
		})

		if runtime.GOOS == "windows" {
			// If running on Windows, wait for notify to be ready, to avoid races.
			time.Sleep(time.Second)
		}

		// Verify that no events have been emitted before the write burst.
		select {
		case <-eventsCh:
			assert.Fail(t, "unexpected event received before write burst")
		default:
			// No event yet, as expected.
		}

		// Perform a burst of writes on the same file; debounce logic should
		// coalesce these into a single notification.
		for range 5 {
			require.NoError(t, os.WriteFile(fp, []byte("data"), 0o600))
		}

		// Expect exactly one event corresponding to the burst.
		select {
		case <-eventsCh:
			// First event received as expected.
		case <-time.After(time.Second):
			assert.Fail(t, "timeout waiting for debounced event")
		}

		// Ensure no additional events arrive for this burst within a window
		// long enough to cover the debounce interval.
		select {
		case <-eventsCh:
			assert.Fail(t, "received more than one event for debounced burst of writes")
		case <-time.After(500 * time.Millisecond):
			// No extra events, as expected.
		}
	})
}
