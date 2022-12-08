/*
Copyright 2022 The Dapr Authors
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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatch(t *testing.T) {
	baseDir := t.TempDir()

	t.Run("watch for file changes", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start watching
		eventCh := make(chan struct{})
		doneCh := make(chan struct{})
		go func() {
			err := Watch(ctx, baseDir, eventCh)
			if errors.Is(err, context.Canceled) {
				doneCh <- struct{}{}
			} else {
				panic(err)
			}
		}()

		// Wait 1s for the watcher to start before touching the file
		time.Sleep(time.Second)

		statusCh := make(chan bool)
		go func() {
			select {
			case <-eventCh:
				statusCh <- true
			case <-time.After(2 * time.Second):
				statusCh <- false
			}
		}()

		touchFile(baseDir, "file1")

		// Expect a successful notification
		if !(<-statusCh) {
			t.Fatalf("did not get event within 2 seconds")
		}

		// Cancel and wait for the watcher to exit
		cancel()
		select {
		case <-doneCh:
			// All good - nop
		case <-time.After(2 * time.Second):
			t.Fatalf("did not stop within 2 seconds")
		}
	})

	t.Run("changes are batched", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start watching
		eventCh := make(chan struct{})
		doneCh := make(chan struct{})
		go func() {
			err := Watch(ctx, baseDir, eventCh)
			if errors.Is(err, context.Canceled) {
				doneCh <- struct{}{}
			} else {
				panic(err)
			}
		}()

		// Wait 1s for the watcher to start before touching the file
		time.Sleep(time.Second)

		statusCh := make(chan bool, 1)
		go func() {
			for {
				select {
				case <-eventCh:
					statusCh <- true
				case <-time.After(2 * time.Second):
					statusCh <- false
					return
				}
			}
		}()

		// Touch the files
		touchFile(baseDir, "file1")
		touchFile(baseDir, "file2")
		touchFile(baseDir, "file3")

		// First message should be true
		if !(<-statusCh) {
			t.Fatalf("did not get event within 2 seconds")
		}

		// Second should be false
		if <-statusCh {
			t.Fatalf("got more than 1 change notification")
		}

		// Repeat
		go func() {
			for {
				select {
				case <-eventCh:
					statusCh <- true
				case <-time.After(2 * time.Second):
					statusCh <- false
					return
				}
			}
		}()
		touchFile(baseDir, "file1")
		touchFile(baseDir, "file2")
		touchFile(baseDir, "file3")

		// First message should be true
		if !(<-statusCh) {
			t.Fatalf("did not get event within 2 seconds")
		}

		// Second should be false
		if <-statusCh {
			t.Fatalf("got more than 1 change notification")
		}

		// Cancel and wait for the watcher to exit
		cancel()
		select {
		case <-doneCh:
			// All good - nop
		case <-time.After(2 * time.Second):
			t.Fatalf("did not stop within 2 seconds")
		}
	})
}

func touchFile(base, name string) {
	path := filepath.Join(base, name)
	err := os.WriteFile(path, []byte("hola"), 0o666)
	if err != nil {
		panic(err)
	}
}
