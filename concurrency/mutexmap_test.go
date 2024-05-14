package concurrency

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMutexMap_Add_Delete(t *testing.T) {
	mm := NewMutexMapString()

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

	t.Run("Clear all mutexes", func(t *testing.T) {
		mm.Lock("key1")
		mm.Unlock("key1")
		mm.Lock("key2")
		mm.Unlock("key2")
		mm.Clear()
		require.Empty(t, mm.items)
	})
}
