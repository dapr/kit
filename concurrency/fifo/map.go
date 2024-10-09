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

package fifo

// Map is a map of mutexes whose locks are acquired in a FIFO order. The map is
// pruned automatically when all locks have been released for a key.
type Map[T comparable] interface {
	Lock(key T)
	Unlock(key T)
}

type mapItem struct {
	ilen  uint64
	mutex *Mutex
}

type fifoMap[T comparable] struct {
	lock  *Mutex
	items map[T]*mapItem
}

func NewMap[T comparable]() Map[T] {
	return &fifoMap[T]{
		lock:  New(),
		items: make(map[T]*mapItem),
	}
}

func (a *fifoMap[T]) Lock(key T) {
	a.lock.Lock()
	m, ok := a.items[key]
	if !ok {
		m = &mapItem{mutex: New()}
		a.items[key] = m
	}
	m.ilen++
	a.lock.Unlock()

	m.mutex.Lock()
}

func (a *fifoMap[T]) Unlock(key T) {
	a.lock.Lock()
	m := a.items[key]
	m.ilen--
	if m.ilen == 0 {
		delete(a.items, key)
	}
	a.lock.Unlock()
	m.mutex.Unlock()
}
