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
)

type MutexMap[T comparable] interface {
	Lock(key T)
	Unlock(key T)
	RLock(key T)
	RUnlock(key T)
	Delete(key T)
	Clear()
	ItemCount() int
}

type mutexMap[T comparable] struct {
	lock  sync.RWMutex
	items map[T]*sync.RWMutex
}

func NewMutexMap[T comparable]() MutexMap[T] {
	return &mutexMap[T]{
		items: make(map[T]*sync.RWMutex),
	}
}

func (a *mutexMap[T]) Lock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	a.lock.RUnlock()
	if !ok {
		a.lock.Lock()
		mutex, ok = a.items[key]
		if !ok {
			mutex = &sync.RWMutex{}
			a.items[key] = mutex
		}
		a.lock.Unlock()
	}
	mutex.Lock()
}

func (a *mutexMap[T]) Unlock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	a.lock.RUnlock()
	if ok {
		mutex.Unlock()
	}
}

func (a *mutexMap[T]) RLock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	a.lock.RUnlock()
	if !ok {
		a.lock.Lock()
		mutex, ok = a.items[key]
		if !ok {
			mutex = &sync.RWMutex{}
			a.items[key] = mutex
		}
		a.lock.Unlock()
	}
	mutex.Lock()
}

func (a *mutexMap[T]) RUnlock(key T) {
	a.lock.RLock()
	mutex, ok := a.items[key]
	a.lock.RUnlock()
	if ok {
		mutex.Unlock()
	}
}

func (a *mutexMap[T]) Delete(key T) {
	a.lock.Lock()
	delete(a.items, key)
	a.lock.Unlock()
}

func (a *mutexMap[T]) Clear() {
	a.lock.Lock()
	a.items = make(map[T]*sync.RWMutex)
	a.lock.Unlock()
}

func (a *mutexMap[T]) ItemCount() int {
	a.lock.Lock()
	defer a.lock.Unlock()
	return len(a.items)
}
