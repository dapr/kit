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
)

// key is the type of the comparable key used to batch events.
type key interface {
	comparable
}

// Batcher is a one to many event batcher. It batches events and sends them to
// the added event channel subscribers. Events are sent to the channels after
// the interval has elapsed. If events with the same key are received within
// the interval, the timer is reset.
type Batcher[T key] struct {
	interval time.Duration
	actives  map[T]clock.Timer
	eventChs []chan<- struct{}

	clock   clock.WithDelayedExecution
	lock    sync.Mutex
	wg      sync.WaitGroup
	closeCh chan struct{}
	closed  atomic.Bool
}

// New creates a new Batcher with the given interval and key type.
func New[T key](interval time.Duration) *Batcher[T] {
	return &Batcher[T]{
		interval: interval,
		actives:  make(map[T]clock.Timer),
		clock:    clock.RealClock{},
		closeCh:  make(chan struct{}),
	}
}

// WithClock sets the clock used by the batcher. Used for testing.
func (b *Batcher[T]) WithClock(clock clock.WithDelayedExecution) {
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

// Batch adds the given key to the batcher. If an event for this key is already
// active, the timer is reset. If the batcher is closed, the key is silently
// dropped.
func (b *Batcher[T]) Batch(key T) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.closed.Load() {
		return
	}

	if active, ok := b.actives[key]; ok {
		if !active.Stop() {
			<-active.C()
		}
		active.Reset(b.interval)
		return
	}

	b.actives[key] = b.clock.AfterFunc(b.interval, func() {
		b.lock.Lock()
		defer b.lock.Unlock()

		b.wg.Add(len(b.eventChs))
		delete(b.actives, key)
		for _, eventCh := range b.eventChs {
			go func(eventCh chan<- struct{}) {
				defer b.wg.Done()
				select {
				case eventCh <- struct{}{}:
				case <-b.closeCh:
				}
			}(eventCh)
		}
	})
}

// Close closes the batcher. It blocks until all events have been sent to the
// subscribers. The batcher will be a no-op after this call.
func (b *Batcher[T]) Close() {
	defer b.wg.Wait()

	// Lock to ensure that no new timers are created.
	b.lock.Lock()
	if b.closed.CompareAndSwap(false, true) {
		close(b.closeCh)
	}
	actives := b.actives
	b.lock.Unlock()

	for _, active := range actives {
		if !active.Stop() {
			<-active.C()
		}
	}

	b.lock.Lock()
	b.actives = nil
	b.lock.Unlock()
}
