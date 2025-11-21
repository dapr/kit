/*
Copyright 2025 The Dapr Authors
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

package loop

import (
	"context"
	"sync"
	"sync/atomic"
)

type Handler[T any] interface {
	Handle(ctx context.Context, t T) error
}

type Interface[T any] interface {
	Run(ctx context.Context) error
	Enqueue(t T)
	Close(t T)
}

type loop[T any] struct {
	factory *Factory[T]

	head *queueSegment[T]
	tail *queueSegment[T]

	handler Handler[T]

	closed atomic.Bool

	closeCh chan struct{}

	lock sync.RWMutex
}

func (l *loop[T]) Run(ctx context.Context) error {
	defer close(l.closeCh)

	current := l.head
	for current != nil {
		// Drain this segment in order. The channel will be closed either:
		//   - when we "roll over" to a new segment, or
		//   - when Close() is called for the final segment.
		for req := range current.ch {
			if err := l.handler.Handle(ctx, req); err != nil {
				return err
			}
		}

		// Move to the next segment, and return this one to the pool.
		next := current.next
		l.putSegment(current)
		current = next
	}

	return nil
}

func (l *loop[T]) Enqueue(req T) {
	l.lock.RLock()

	if l.closed.Load() {
		l.lock.RUnlock()
		return
	}

	// First try to send to the current tail segment without blocking.
	select {
	case l.tail.ch <- req:
		l.lock.RUnlock()
		return
	default:
		l.lock.RUnlock()
	}

	// Tail is full; need to acquire write lock to roll over. If no longer full
	// (lost race, another goroutine rolled over first), don't expand.

	l.lock.Lock()
	defer l.lock.Unlock()

	if l.closed.Load() {
		// Closed while we were waiting for the lock.
		return
	}

	// Try again to send to the tail; if successful, another goroutine must
	// have rolled over for us.
	select {
	case l.tail.ch <- req:
	default:
		// Tail is full: create a new segment, link it, close the old tail, and
		// send into the new tail.
		newSeg := l.getSegment()
		l.tail.next = newSeg
		close(l.tail.ch)
		l.tail = newSeg
		l.tail.ch <- req
	}
}

func (l *loop[T]) Close(req T) {
	l.lock.Lock()
	if l.closed.Load() {
		// Already closed; just unlock and wait for Run to finish.
		l.lock.Unlock()
		<-l.closeCh
		return
	}
	l.closed.Store(true)

	// Enqueue the final request; if the tail is full, roll over as in Enqueue.
	select {
	case l.tail.ch <- req:
	default:
		newSeg := l.getSegment()
		l.tail.next = newSeg
		close(l.tail.ch)
		l.tail = newSeg
		l.tail.ch <- req
	}

	// No more items will be enqueued; close the tail to signal completion.
	close(l.tail.ch)
	l.lock.Unlock()

	// Wait for Run to finish draining everything.
	<-l.closeCh
}
