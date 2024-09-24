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

// Map is a simple _typed_ map which is safe for concurrent use.
// Favoured over sync.Map as it is typed.
type Map[K comparable, T any] interface {
	Clear()
	Delete(key K)
	Load(key K) (T, bool)
	LoadAndDelete(key K) (T, bool)
	Range(fn func(key K, value T) bool)
	Store(key K, value T)
}

type mapimpl[K comparable, T any] struct {
	lock sync.RWMutex
	m    map[K]T
}

func NewMap[K comparable, T any]() Map[K, T] {
	return &mapimpl[K, T]{m: make(map[K]T)}
}

func (m *mapimpl[K, T]) Clear() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.m = make(map[K]T)
}

func (m *mapimpl[K, T]) Delete(k K) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.m, k)
}

func (m *mapimpl[K, T]) Load(k K) (T, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	v, ok := m.m[k]
	return v, ok
}

func (m *mapimpl[K, T]) LoadAndDelete(k K) (T, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	v, ok := m.m[k]
	delete(m.m, k)
	return v, ok
}

func (m *mapimpl[K, T]) Range(fn func(K, T) bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	for k, v := range m.m {
		if !fn(k, v) {
			break
		}
	}
}

func (m *mapimpl[K, T]) Store(k K, v T) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.m[k] = v
}
