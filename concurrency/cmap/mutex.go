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

package cmap

import (
	"sync"
)

// Mutex is an interface that defines a thread-safe map with keys of type T associated to
// read-write mutexes (sync.RWMutex), allowing for granular locking on a per-key basis.
// This can be useful for scenarios where fine-grained concurrency control is needed.
//
// Methods:
// - Lock(key T): Acquires an exclusive lock on the mutex associated with the given key.
// - Unlock(key T): Releases the exclusive lock on the mutex associated with the given key.
// - RLock(key T): Acquires a read lock on the mutex associated with the given key.
// - RUnlock(key T): Releases the read lock on the mutex associated with the given key.
// - Delete(key T): Removes the mutex associated with the given key from the map.
// - Clear(): Removes all mutexes from the map.
// - ItemCount() int: Returns the number of items (mutexes) in the map.
// - DeleteUnlock(key T): Removes the mutex associated with the given key from the map and releases the lock.
// - DeleteRUnlock(key T): Removes the mutex associated with the given key from the map and releases the read lock.
type Mutex[T comparable] interface {
	Lock(key T)
	Unlock(key T)
	RLock(key T)
	RUnlock(key T)
	Delete(key T)
	Clear()
	ItemCount() int
	DeleteUnlock(key T)
	DeleteRUnlock(key T)
}

type mutex[T comparable] struct {
	lock  sync.RWMutex
	items map[T]*sync.RWMutex
}

func NewMutex[T comparable]() Mutex[T] {
	return &mutex[T]{
		items: make(map[T]*sync.RWMutex),
	}
}

func (a *mutex[T]) Lock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	a.lock.RUnlock()
	if ok {
		mutex.Lock()
		return
	}

	a.lock.Lock()
	mutex, ok = a.items[key]
	if !ok {
		mutex = &sync.RWMutex{}
		a.items[key] = mutex
	}
	a.lock.Unlock()
	mutex.Lock()
}

func (a *mutex[T]) Unlock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	if ok {
		mutex.Unlock()
	}
	a.lock.RUnlock()
}

func (a *mutex[T]) RLock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	a.lock.RUnlock()

	if ok {
		mutex.RLock()
		return
	}

	a.lock.Lock()
	mutex, ok = a.items[key]
	if !ok {
		mutex = &sync.RWMutex{}
		a.items[key] = mutex
	}
	a.lock.Unlock()
	mutex.RLock()
}

func (a *mutex[T]) RUnlock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	if ok {
		mutex.RUnlock()
	}
	a.lock.RUnlock()
}

func (a *mutex[T]) Delete(key T) {
	a.lock.Lock()
	delete(a.items, key)
	a.lock.Unlock()
}

func (a *mutex[T]) DeleteUnlock(key T) {
	a.lock.Lock()
	mutex, ok := a.items[key]
	if ok {
		mutex.Unlock()
	}
	delete(a.items, key)
	a.lock.Unlock()
}

func (a *mutex[T]) DeleteRUnlock(key T) {
	a.lock.Lock()
	mutex, ok := a.items[key]
	if ok {
		mutex.RUnlock()
	}
	delete(a.items, key)
	a.lock.Unlock()
}

func (a *mutex[T]) Clear() {
	a.lock.Lock()
	clear(a.items)
	a.lock.Unlock()
}

func (a *mutex[T]) ItemCount() int {
	a.lock.Lock()
	defer a.lock.Unlock()
	return len(a.items)
}
