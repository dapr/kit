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
	"sync/atomic"

	"github.com/dapr/kit/concurrency/fifo"
)

type HandlerFunc[T any] func(context.Context, T) error

type Options[T any] struct {
	Handler HandlerFunc[T]
}

type Loop[T any] struct {
	queue   chan T
	handler HandlerFunc[T]

	closeCh chan struct{}
	closed  atomic.Bool
	lock    fifo.Mutex
}

func New[T any](opts Options[T]) *Loop[T] {
	return &Loop[T]{
		queue:   make(chan T, 1),
		closeCh: make(chan struct{}),
		handler: opts.Handler,
	}
}

func (l *Loop[T]) Run(ctx context.Context) error {
	defer close(l.closeCh)

	for {
		var req T
		select {
		case req = <-l.queue:
		case <-ctx.Done():
			return ctx.Err()
		}

		if err := l.handler(ctx, req); err != nil {
			return err
		}
	}
}

func (l *Loop[T]) Enqueue(req T) {
	select {
	case l.queue <- req:
	case <-l.closeCh:
	}
}
