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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMutexMap_Add_Delete(t *testing.T) {
	mm := NewMutexMap[string]().(*mutexMap[string])

	t.Run("New mutex map", func(t *testing.T) {
		require.NotNil(t, mm)
		require.NotNil(t, mm.items)
		require.Empty(t, mm.items)
	})

	t.Run("Lock and unlock mutex", func(t *testing.T) {
		mm.Lock("key1")
		_, ok := mm.items["key1"]
		require.True(t, ok)
		mm.Unlock("key1")
	})

	t.Run("Concurrently lock and unlock mutexes", func(t *testing.T) {
		var counter int
		var wg sync.WaitGroup

		numGoroutines := 10
		wg.Add(numGoroutines)

		// Concurrently lock and unlock for each key
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				mm.Lock("key1")
				counter++
				mm.Unlock("key1")
			}()
		}
		wg.Wait()

		require.Equal(t, 10, counter)
	})

	t.Run("RLock and RUnlock mutex", func(t *testing.T) {
		mm.RLock("key1")
		_, ok := mm.items["key1"]
		require.True(t, ok)
		mm.RUnlock("key1")
	})

	t.Run("Concurrently RLock and RUnlock mutexes", func(t *testing.T) {
		var counter int
		var wg sync.WaitGroup

		numGoroutines := 10
		wg.Add(numGoroutines)

		// Concurrently RLock and RUnlock for each key
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				mm.RLock("key1")
				counter++
				mm.RUnlock("key1")
			}()
		}
		wg.Wait()

		require.Equal(t, 10, counter)
	})

	t.Run("Delete mutex", func(t *testing.T) {
		mm.Lock("key1")
		mm.Unlock("key1")
		mm.Delete("key1")
		_, ok := mm.items["key1"]
		require.False(t, ok)
	})

	t.Run("Clear all mutexes, and check item count", func(t *testing.T) {
		mm.Lock("key1")
		mm.Unlock("key1")
		mm.Lock("key2")
		mm.Unlock("key2")

		require.Equal(t, 2, mm.ItemCount())

		mm.Clear()
		require.Empty(t, mm.items)
	})
}
