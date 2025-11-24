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

import "sync"

type Factory[T any] struct {
	size uint64

	segPool  sync.Pool
	loopPool sync.Pool
}

func New[T any](size uint64) *Factory[T] {
	f := &Factory[T]{
		size: size,
		segPool: sync.Pool{
			New: func() any {
				return new(queueSegment[T])
			},
		},
	}

	f.loopPool = sync.Pool{
		New: func() any {
			return &loop[T]{
				factory: f,
			}
		},
	}

	return f
}

func (f *Factory[T]) NewLoop(h Handler[T]) Interface[T] {
	l := f.loopPool.Get().(*loop[T])

	seg := l.getSegment()
	l.head = seg
	l.tail = seg
	l.closeCh = make(chan struct{})
	l.handler = h
	l.closed.Store(false)

	return l
}

func (f *Factory[T]) CacheLoop(l Interface[T]) {
	f.loopPool.Put(l)
}
