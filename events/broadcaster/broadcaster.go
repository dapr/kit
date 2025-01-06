/*
Copyright 2024 The Dapr Authors
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

package broadcaster

import (
	"context"
	"sync"
	"sync/atomic"
)

const bufferSize = 10

type eventCh[T any] struct {
	id           uint64
	ch           chan<- T
	closeEventCh chan struct{}
}

type Broadcaster[T any] struct {
	eventChs  []*eventCh[T]
	currentID uint64

	lock    sync.Mutex
	wg      sync.WaitGroup
	closeCh chan struct{}
	closed  atomic.Bool
}

// New creates a new Broadcaster with the given interval and key type.
func New[T any]() *Broadcaster[T] {
	return &Broadcaster[T]{
		closeCh: make(chan struct{}),
	}
}

// Subscribe adds a new event channel subscriber. If the batcher is closed, the
// subscriber is silently dropped.
func (b *Broadcaster[T]) Subscribe(ctx context.Context, ch ...chan<- T) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, c := range ch {
		b.subscribe(ctx, c)
	}
}

func (b *Broadcaster[T]) subscribe(ctx context.Context, ch chan<- T) {
	if b.closed.Load() {
		return
	}

	id := b.currentID
	b.currentID++
	bufferedCh := make(chan T, bufferSize)
	closeEventCh := make(chan struct{})
	b.eventChs = append(b.eventChs, &eventCh[T]{
		id:           id,
		ch:           bufferedCh,
		closeEventCh: closeEventCh,
	})

	b.wg.Add(1)
	go func() {
		defer func() {
			close(closeEventCh)

			b.lock.Lock()
			for i, eventCh := range b.eventChs {
				if eventCh.id == id {
					b.eventChs = append(b.eventChs[:i], b.eventChs[i+1:]...)
					break
				}
			}
			b.lock.Unlock()
			b.wg.Done()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-b.closeCh:
				return
			case ch <- <-bufferedCh:
			}
		}
	}()
}

// Broadcast sends the given value to all subscribers.
func (b *Broadcaster[T]) Broadcast(value T) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed.Load() {
		return
	}
	for _, ev := range b.eventChs {
		select {
		case <-ev.closeEventCh:
		case ev.ch <- value:
		case <-b.closeCh:
		}
	}
}

// Close closes the Broadcaster. It blocks until all events have been sent to
// the subscribers. The Broadcaster will be a no-op after this call.
func (b *Broadcaster[T]) Close() {
	defer b.wg.Wait()
	b.lock.Lock()
	if b.closed.CompareAndSwap(false, true) {
		close(b.closeCh)
	}
	b.lock.Unlock()
}
