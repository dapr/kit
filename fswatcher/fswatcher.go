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
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/dapr/kit/events/batcher"
)

// Options are the options for the FSWatcher.
type Options struct {
	// Targets is a list of directories to watch for changes.
	Targets []string

	// Interval is the interval to wait before sending a notification after a file has changed.
	// Default to 500ms.
	Interval *time.Duration
}

// FSWatcher watches for changes to a directory on the filesystem and sends a notification to eventCh every time a file in the folder is changed.
// Although it's possible to watch for individual files, that's not recommended; watch for the file's parent folder instead.
// That is because, like in Kubernetes which uses system links on mounted volumes, the file may be deleted and recreated with a different inode.
// Note that changes are batched for 0.5 seconds before notifications are sent as events on a single file often come in batches.
type FSWatcher struct {
	w       *fsnotify.Watcher
	running atomic.Bool
	batcher *batcher.Batcher[string, struct{}]
}

func New(opts Options) (*FSWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	for _, target := range opts.Targets {
		if err = w.Add(target); err != nil {
			return nil, fmt.Errorf("failed to add target %s: %w", target, err)
		}
	}

	interval := time.Millisecond * 500
	if opts.Interval != nil {
		interval = *opts.Interval
	}
	if interval < 0 {
		return nil, errors.New("interval must be positive")
	}

	return &FSWatcher{
		w: w,
		// Often the case, writes to files are not atomic and involve multiple file system events.
		// We want to hold off on sending events until we are sure that the file has been written to completion. We do this by waiting for a period of time after the last event has been received for a file name.
		batcher: batcher.New[string, struct{}](batcher.Options{
			Interval: interval,
		}),
	}, nil
}

func (f *FSWatcher) Run(ctx context.Context, eventCh chan<- struct{}) error {
	if !f.running.CompareAndSwap(false, true) {
		return errors.New("watcher already running")
	}
	defer f.batcher.Close()

	f.batcher.Subscribe(ctx, eventCh)

	for {
		select {
		case <-ctx.Done():
			return f.w.Close()
		case err := <-f.w.Errors:
			return errors.Join(fmt.Errorf("watcher error: %w", err), f.w.Close())
		case event := <-f.w.Events:
			f.batcher.Batch(event.Name, struct{}{})
		}
	}
}
