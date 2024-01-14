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

package batcher

import (
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/utils/clock"

	"github.com/dapr/kit/events/queue"
)

// Batcher is a one to many event batcher. It batches events and sends them to
// the added event channel subscribers. Events are sent to the channels after
// the interval has elapsed. If events with the same key are received within
// the interval, the timer is reset.
type Batcher[T comparable] struct {
	interval time.Duration
	eventChs []chan<- struct{}
	queue    *queue.Processor[T, *item[T]]

	clock   clock.Clock
	lock    sync.Mutex
	wg      sync.WaitGroup
	closeCh chan struct{}
	closed  atomic.Bool
}

// New creates a new Batcher with the given interval and key type.
func New[T comparable](interval time.Duration) *Batcher[T] {
	b := &Batcher[T]{
		interval: interval,
		clock:    clock.RealClock{},
		closeCh:  make(chan struct{}),
	}

	b.queue = queue.NewProcessor[T, *item[T]](b.execute)

	return b
}

// WithClock sets the clock used by the batcher. Used for testing.
func (b *Batcher[T]) WithClock(clock clock.Clock) {
	b.queue.WithClock(clock)
	b.clock = clock
}

// Subscribe adds a new event channel subscriber. If the batcher is closed, the
// subscriber is silently dropped.
func (b *Batcher[T]) Subscribe(eventCh ...chan<- struct{}) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed.Load() {
		return
	}
	b.eventChs = append(b.eventChs, eventCh...)
}

func (b *Batcher[T]) execute(_ *item[T]) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed.Load() {
		return
	}
	b.wg.Add(len(b.eventChs))
	for _, eventCh := range b.eventChs {
		go func(eventCh chan<- struct{}) {
			defer b.wg.Done()
			select {
			case eventCh <- struct{}{}:
			case <-b.closeCh:
			}
		}(eventCh)
	}
}

// Batch adds the given key to the batcher. If an event for this key is already
// active, the timer is reset. If the batcher is closed, the key is silently
// dropped.
func (b *Batcher[T]) Batch(key T) {
	b.queue.Enqueue(&item[T]{
		key: key,
		ttl: b.clock.Now().Add(b.interval),
	})
}

// Close closes the batcher. It blocks until all events have been sent to the
// subscribers. The batcher will be a no-op after this call.
func (b *Batcher[T]) Close() {
	defer b.wg.Wait()
	b.lock.Lock()
	if b.closed.CompareAndSwap(false, true) {
		close(b.closeCh)
	}
	b.lock.Unlock()
	b.queue.Close()
}

// item implements queue.queueable.
type item[T comparable] struct {
	key T
	ttl time.Time
}

func (b *item[T]) Key() T {
	return b.key
}

func (b *item[T]) ScheduledTime() time.Time {
	return b.ttl
}
