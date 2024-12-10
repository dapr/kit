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

package batcher

import (
	"sync"
	"sync/atomic"
	"time"
)

// Singular is a Batcher which batches events and treats them as all the same
// key, behaving like a singular batched queue.
type Singular struct {
	b       *Batcher[int]
	closeCh chan struct{}
	closed  atomic.Bool
	wg      sync.WaitGroup
}

func NewSingular(interval time.Duration) *Singular {
	return &Singular{
		b:       New[int](interval),
		closeCh: make(chan struct{}),
	}
}

func (s *Singular) Subscribe(fn func()) {
	ch := make(chan struct{})
	s.b.Subscribe(ch)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ch:
			case <-s.closeCh:
				return
			}

			fn()
		}
	}()
}

func (s *Singular) Batch() {
	s.b.Batch(0)
}

func (s *Singular) Close() {
	defer s.wg.Wait()
	if s.closed.CompareAndSwap(false, true) {
		close(s.closeCh)
	}
	s.b.Close()
}
