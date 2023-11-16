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
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/dapr/kit/events/batcher"
	"github.com/dapr/kit/ptr"
)

func TestFSWatcher(t *testing.T) {
	runWatcher := func(t *testing.T, opts Options, bacher *batcher.Batcher[string]) <-chan struct{} {
		t.Helper()

		f, err := New(opts)
		require.NoError(t, err)

		if bacher != nil {
			f.WithBatcher(bacher)
		}

		errCh := make(chan error)
		ctx, cancel := context.WithCancel(context.Background())
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
		runWatcher(t, Options{}, nil)
	})

	t.Run("creating fswatcher with 0 interval should not error", func(t *testing.T) {
		_, err := New(Options{
			Interval: ptr.Of(time.Duration(0)),
		})
		require.NoError(t, err)
	})

	t.Run("creating fswatcher with negative interval should error", func(t *testing.T) {
		_, err := New(Options{
			Interval: ptr.Of(time.Duration(-1)),
		})
		require.Error(t, err)
	})

	t.Run("running Run twice should error", func(t *testing.T) {
		fs, err := New(Options{})
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
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
		require.NoError(t, os.WriteFile(fp, []byte{}, 0o644))
		eventsCh := runWatcher(t, Options{
			Targets:  []string{fp},
			Interval: ptr.Of(time.Duration(1)),
		}, nil)
		assert.Empty(t, eventsCh)

		if runtime.GOOS == "windows" {
			// If running in windows, wait for notify to be ready.
			time.Sleep(time.Second)
		}
		require.NoError(t, os.WriteFile(fp, []byte{}, 0o644))

		select {
		case <-eventsCh:
		case <-time.After(time.Second):
			assert.Fail(t, "timeout waiting for event")
		}
	})

	t.Run("should fire 2 events when event occurs on 2 file target", func(t *testing.T) {
		fp1 := filepath.Join(t.TempDir(), "test.txt")
		fp2 := filepath.Join(t.TempDir(), "test.txt")
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o644))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o644))
		eventsCh := runWatcher(t, Options{
			Targets:  []string{fp1, fp2},
			Interval: ptr.Of(time.Duration(1)),
		}, nil)
		assert.Empty(t, eventsCh)
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o644))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o644))
		for i := 0; i < 2; i++ {
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
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o644))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o644))
		eventsCh := runWatcher(t, Options{
			Targets:  []string{fp1, fp2},
			Interval: ptr.Of(time.Duration(1)),
		}, nil)
		if runtime.GOOS == "windows" {
			// If running in windows, wait for notify to be ready.
			time.Sleep(time.Second)
		}
		assert.Empty(t, eventsCh)
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o644))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o644))
		for i := 0; i < 2; i++ {
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
			Targets:  []string{dir1, dir2},
			Interval: ptr.Of(time.Duration(1)),
		}, nil)
		assert.Empty(t, eventsCh)
		require.NoError(t, os.WriteFile(fp1, []byte{}, 0o644))
		require.NoError(t, os.WriteFile(fp2, []byte{}, 0o644))
		for i := 0; i < 2; i++ {
			select {
			case <-eventsCh:
			case <-time.After(time.Second):
				assert.Fail(t, "timeout waiting for event")
			}
		}
	})

	t.Run("should batch events of the same file for multiple events", func(t *testing.T) {
		clock := clocktesting.NewFakeClock(time.Time{})
		batcher := batcher.New[string](time.Millisecond * 500)
		batcher.WithClock(clock)
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		fp1 := filepath.Join(dir1, "test1.txt")
		fp2 := filepath.Join(dir2, "test2.txt")
		eventsCh := runWatcher(t, Options{Targets: []string{dir1, dir2}}, batcher)
		assert.Empty(t, eventsCh)

		if runtime.GOOS == "windows" {
			// If running in windows, wait for notify to be ready.
			time.Sleep(time.Second)
		}

		for i := 0; i < 10; i++ {
			require.NoError(t, os.WriteFile(fp1, []byte{}, 0o644))
			require.NoError(t, os.WriteFile(fp2, []byte{}, 0o644))
		}

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond*10)

		select {
		case <-eventsCh:
			assert.Fail(t, "unexpected event")
		case <-time.After(time.Millisecond * 10):
		}

		clock.Step(time.Millisecond * 250)

		for i := 0; i < 10; i++ {
			require.NoError(t, os.WriteFile(fp1, []byte{}, 0o644))
			require.NoError(t, os.WriteFile(fp2, []byte{}, 0o644))
		}

		select {
		case <-eventsCh:
			assert.Fail(t, "unexpected event")
		case <-time.After(time.Millisecond * 10):
		}

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond*10)
		clock.Step(time.Millisecond * 500)

		for i := 0; i < 2; i++ {
			select {
			case <-eventsCh:
			case <-time.After(time.Second):
				assert.Fail(t, "timeout waiting for event")
			}
			clock.Step(1)
		}
	})
}
