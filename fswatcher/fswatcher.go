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
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/events/loop"
)

// Options are the options for the FSWatcher.
type Options struct {
	// Targets is a list of directories to watch for changes.
	Targets []string
}

// FSWatcher watches for changes to a directory on the filesystem and sends a notification to eventCh every time a file in the folder is changed.
// Although it's possible to watch for individual files, that's not recommended; watch for the file's parent folder instead.
// That is because, like in Kubernetes which uses system links on mounted volumes, the file may be deleted and recreated with a different inode.
type FSWatcher struct {
	w       *fsnotify.Watcher
	running atomic.Bool
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

	return &FSWatcher{w: w}, nil
}

func (f *FSWatcher) Run(ctx context.Context, eventCh chan<- struct{}) error {
	if !f.running.CompareAndSwap(false, true) {
		return errors.New("watcher already running")
	}

	factory := loop.New[event](64)
	l := factory.NewLoop(&handler{eventCh: eventCh})

	// debounce holds a per-file timer that fires after a short idle window.
	// A single logical write (open+O_TRUNC → write → close) generates several
	// OS-level inotify events in quick succession; coalescing them into one
	// notification ensures consumers never see a transiently-empty file and
	// never receive spurious duplicate signals.
	const debounceInterval = 5 * time.Millisecond
	var (
		debounceMu sync.Mutex
		debounce   = make(map[string]*time.Timer)
	)
	enqueueDebounced := func(name string) {
		debounceMu.Lock()
		defer debounceMu.Unlock()
		if t, ok := debounce[name]; ok {
			t.Stop()
		}
		debounce[name] = time.AfterFunc(debounceInterval, func() {
			// Lock only long enough to remove the entry, then release
			// before calling l.Enqueue so we never hold debounceMu
			// across an external call.
			debounceMu.Lock()
			delete(debounce, name)
			debounceMu.Unlock()
			l.Enqueue(event{name: name})
		})
	}

	return concurrency.NewRunnerManager(
		l.Run,
		func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					l.Close(event{shutdown: true})
					return f.w.Close()
				case err, ok := <-f.w.Errors:
					if !ok {
						l.Close(event{shutdown: true})
						return nil
					}
					if err == nil {
						l.Close(event{shutdown: true})
						return nil
					}
					l.Close(event{shutdown: true})
					return errors.Join(fmt.Errorf("watcher error: %w", err), f.w.Close())
				case fsEvent, ok := <-f.w.Events:
					if !ok {
						l.Close(event{shutdown: true})
						return nil
					}
					enqueueDebounced(fsEvent.Name)
				}
			}
		}).Run(ctx)
}
