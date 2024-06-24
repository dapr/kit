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
	"context"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/utils/clock"

	"github.com/dapr/kit/events/queue"
)

type eventCh[T any] struct {
	id int
	ch chan<- T
}

// Batcher is a one to many event batcher. It batches events and sends them to
// the added event channel subscribers. Events are sent to the channels after
// the interval has elapsed. If events with the same key are received within
// the interval, the timer is reset.
type Batcher[K comparable, T any] struct {
	interval  time.Duration
	eventChs  []*eventCh[T]
	queue     *queue.Processor[K, *item[K, T]]
	currentID int

	clock   clock.Clock
	lock    sync.Mutex
	wg      sync.WaitGroup
	closeCh chan struct{}
	closed  atomic.Bool
}

// New creates a new Batcher with the given interval and key type.
func New[K comparable, T any](interval time.Duration) *Batcher[K, T] {
	b := &Batcher[K, T]{
		interval: interval,
		clock:    clock.RealClock{},
		closeCh:  make(chan struct{}),
	}

	b.queue = queue.NewProcessor[K, *item[K, T]](b.execute)

	return b
}

// WithClock sets the clock used by the batcher. Used for testing.
func (b *Batcher[K, T]) WithClock(clock clock.Clock) {
	b.queue.WithClock(clock)
	b.clock = clock
}

// Subscribe adds a new event channel subscriber. If the batcher is closed, the
// subscriber is silently dropped.
func (b *Batcher[K, T]) Subscribe(ctx context.Context, ch ...chan<- T) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, c := range ch {
		b.subscribe(ctx, c)
	}
}

func (b *Batcher[K, T]) subscribe(ctx context.Context, ch chan<- T) {
	if b.closed.Load() {
		return
	}

	id := b.currentID
	b.currentID++
	bufferedCh := make(chan T, 50)
	b.eventChs = append(b.eventChs, &eventCh[T]{
		id: id,
		ch: bufferedCh,
	})

	b.wg.Add(1)
	go func() {
		defer func() {
			b.lock.Lock()
			close(ch)
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
			case env := <-bufferedCh:
				select {
				case ch <- env:
				case <-ctx.Done():
				case <-b.closeCh:
				}
			}
		}
	}()
}

func (b *Batcher[K, T]) execute(i *item[K, T]) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.closed.Load() {
		return
	}
	for _, ev := range b.eventChs {
		select {
		case ev.ch <- i.value:
		case <-b.closeCh:
		}
	}
}

// Batch adds the given key to the batcher. If an event for this key is already
// active, the timer is reset. If the batcher is closed, the key is silently
// dropped.
func (b *Batcher[K, T]) Batch(key K, value T) {
	b.queue.Enqueue(&item[K, T]{
		key:   key,
		value: value,
		ttl:   b.clock.Now().Add(b.interval),
	})
}

// Close closes the batcher. It blocks until all events have been sent to the
// subscribers. The batcher will be a no-op after this call.
func (b *Batcher[K, T]) Close() {
	defer b.wg.Wait()
	b.queue.Close()
	b.lock.Lock()
	if b.closed.CompareAndSwap(false, true) {
		close(b.closeCh)
	}
	b.lock.Unlock()
}

// item implements queue.queueable.
type item[K comparable, T any] struct {
	key   K
	value T
	ttl   time.Time
}

func (b *item[K, T]) Key() K {
	return b.key
}

func (b *item[K, T]) ScheduledTime() time.Time {
	return b.ttl
}
