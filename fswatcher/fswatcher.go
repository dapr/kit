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

	return concurrency.NewRunnerManager(
		l.Run,
		func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					l.Close(event{shutdown: true})
					return f.w.Close()
				case err := <-f.w.Errors:
					l.Close(event{shutdown: true})
					return errors.Join(fmt.Errorf("watcher error: %w", err), f.w.Close())
				case fsEvent := <-f.w.Events:
					l.Enqueue(event{name: fsEvent.Name})
				}
			}
		}).Run(ctx)
}
