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

package fake

import (
	"context"

	"github.com/dapr/kit/events/loop"
)

type Fake[T any] struct {
	runFn     func(context.Context) error
	enqueueFn func(T)
	closeFn   func(T)
}

func New[T any]() *Fake[T] {
	return &Fake[T]{
		runFn:     func(context.Context) error { return nil },
		enqueueFn: func(T) {},
		closeFn:   func(T) {},
	}
}

func (f *Fake[T]) WithRun(fn func(context.Context) error) *Fake[T] {
	f.runFn = fn
	return f
}

func (f *Fake[T]) WithEnqueue(fn func(T)) *Fake[T] {
	f.enqueueFn = fn
	return f
}

func (f *Fake[T]) WithClose(fn func(T)) *Fake[T] {
	f.closeFn = fn
	return f
}

func (f *Fake[T]) Run(ctx context.Context) error {
	return f.runFn(ctx)
}

func (f *Fake[T]) Enqueue(t T) {
	f.enqueueFn(t)
}

func (f *Fake[T]) Close(t T) {
	f.closeFn(t)
}

func (f *Fake[T]) Reset(loop.Handler[T], uint64) loop.Interface[T] {
	return f
}
