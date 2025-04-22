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

package slice

import "sync"

// Slice is a concurrent safe types slice
type Slice[T any] interface {
	Append(items ...T) int
	Len() int
	Slice() []T
	Store(items ...T)
}

type slice[T any] struct {
	lock sync.RWMutex
	data []T
}

func New[T any]() Slice[T] {
	return new(slice[T])
}

func (s *slice[T]) Append(items ...T) int {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data = append(s.data, items...)
	return len(s.data)
}

func (s *slice[T]) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.data)
}

func (s *slice[T]) Slice() []T {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.data
}

func (s *slice[T]) Store(items ...T) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data = items
}
