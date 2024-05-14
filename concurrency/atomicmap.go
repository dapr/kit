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

package concurrency

import (
	"sync"

	"golang.org/x/exp/constraints"
)

type AtomicValue[T constraints.Integer] struct {
	lock  sync.RWMutex
	value T
}

func (a *AtomicValue[T]) Load() T {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.value
}

func (a *AtomicValue[T]) Store(v T) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.value = v
}

func (a *AtomicValue[T]) Add(v T) T {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.value += v
	return a.value
}

type AtomicMap[K comparable, T constraints.Integer] struct {
	lock  sync.RWMutex
	items map[K]*AtomicValue[T]
}

func NewAtomicMapStringInt64() *AtomicMap[string, int64] {
	return &AtomicMap[string, int64]{
		items: make(map[string]*AtomicValue[int64]),
	}
}

func NewAtomicMapStringInt32() *AtomicMap[string, int32] {
	return &AtomicMap[string, int32]{
		items: make(map[string]*AtomicValue[int32]),
	}
}

func NewAtomicMapStringUint64() *AtomicMap[string, uint64] {
	return &AtomicMap[string, uint64]{
		items: make(map[string]*AtomicValue[uint64]),
	}
}

func NewAtomicMapStringUint32() *AtomicMap[string, uint32] {
	return &AtomicMap[string, uint32]{
		items: make(map[string]*AtomicValue[uint32]),
	}
}

func (a *AtomicMap[K, T]) Get(key K) (*AtomicValue[T], bool) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	item, ok := a.items[key]
	if !ok {
		return nil, false
	}
	return item, true
}

func (a *AtomicMap[K, T]) GetOrCreate(key K, createT T) *AtomicValue[T] {
	a.lock.RLock()
	item, ok := a.items[key]
	a.lock.RUnlock()
	if !ok {
		a.lock.Lock()
		// Double-check the key exists to avoid race condition
		item, ok = a.items[key]
		if !ok {
			item = &AtomicValue[T]{value: createT}
			a.items[key] = item
		}
		a.lock.Unlock()
	}
	return item
}

func (a *AtomicMap[K, T]) Delete(key K) {
	a.lock.Lock()
	delete(a.items, key)
	a.lock.Unlock()
}

func (a *AtomicMap[K, T]) ForEach(fn func(key K, value *AtomicValue[T])) {
	a.lock.RLock()
	defer a.lock.RUnlock()
	for k, v := range a.items {
		fn(k, v)
	}
}

func (a *AtomicMap[K, T]) Clear() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.items = make(map[K]*AtomicValue[T])
}
