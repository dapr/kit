package fswatcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch for changes to a directory on the filesystem and sends a notification to eventCh every time a file in the folder is changed.
// Although it's possible to watch for individual files, that's not recommended; watch for the file's parent folder instead.
// Note that changes are batched for 0.5 seconds before notifications are sent
func Watch(ctx context.Context, dir string, eventCh chan<- struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	err = watcher.Add(dir)
	if err != nil {
		return fmt.Errorf("watcher error: %w", err)
	}

	batchCh := make(chan struct{}, 1)
	defer close(batchCh)

	for {
		select {
		// Watch for events
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Write == fsnotify.Write {
				if strings.Contains(event.Name, dir) {
					// Batch the change
					select {
					case batchCh <- struct{}{}:
						go func() {
							time.Sleep(500 * time.Millisecond)
							<-batchCh
							eventCh <- struct{}{}
						}()
					default:
						// There's already a change in the batch - nop
					}
				}
			}

		// Abort in case of errors
		case err = <-watcher.Errors:
			return fmt.Errorf("watcher listen error: %w", err)

		// Stop on context canceled
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
