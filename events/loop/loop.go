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
)

type Handler[T any] interface {
	Handle(context.Context, T) error
}

type Interface[T any] interface {
	Run(context.Context) error
	Enqueue(T)
	Close(T)
	Reset(h Handler[T], size uint64) Interface[T]
}

type loop[T any] struct {
	queue   chan T
	handler Handler[T]

	closed  bool
	closeCh chan struct{}
	lock    sync.RWMutex
}

func New[T any](h Handler[T], size uint64) Interface[T] {
	return &loop[T]{
		queue:   make(chan T, size),
		handler: h,
		closeCh: make(chan struct{}),
	}
}

func Empty[T any]() Interface[T] {
	return new(loop[T])
}

func (l *loop[T]) Run(ctx context.Context) error {
	defer close(l.closeCh)

	for {
		req, ok := <-l.queue
		if !ok {
			return nil
		}

		if err := l.handler.Handle(ctx, req); err != nil {
			return err
		}
	}
}

func (l *loop[T]) Enqueue(req T) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	if l.closed {
		return
	}

	select {
	case l.queue <- req:
	case <-l.closeCh:
	}
}

func (l *loop[T]) Close(req T) {
	l.lock.Lock()
	l.closed = true
	l.queue <- req
	close(l.queue)
	l.lock.Unlock()
	<-l.closeCh
}

func (l *loop[T]) Reset(h Handler[T], size uint64) Interface[T] {
	if l == nil {
		return New[T](h, size)
	}

	l.closed = false
	l.closeCh = make(chan struct{})
	l.handler = h

	// TODO: @joshvanl: use a ring buffer so that we don't need to reallocate and
	// improve performance.
	l.queue = make(chan T, size)

	return l
}
