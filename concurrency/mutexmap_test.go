package concurrency

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMutexMap_Add_Delete(t *testing.T) {

	mm := NewMutexMap()

	t.Run("New mutex map", func(t *testing.T) {
		require.NotNil(t, mm)
		require.NotNil(t, mm.mutex)
		require.Len(t, mm.mutex, 0)
	})

	t.Run("Add mutex", func(t *testing.T) {
		mm.Add("key1")
		require.Len(t, mm.mutex, 1)
		_, ok := mm.mutex["key1"]
		require.True(t, ok)
	})


	t.Run("Delete mutex", func(t *testing.T) {
		mm.Delete("key1")
		require.Len(t, mm.mutex, 0)
		_, ok := mm.mutex["key1"]
		require.False(t, ok)
	})

	t.Run("Concurrently add and delete mutexes", func(t *testing.T){
		numGoroutines := 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Concurrently add and delete keys
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for _, key := range keys {
					mm.Add(key)
					mm.Delete(key)
				}
			}()
		}
		wg.Wait()

		// Additional check that all keys have been deleted
		for _, key := range keys {
			_, ok := mm.mutex[key]
			require.False(t, ok)
		}
	})

	t.Run("Lock & Unlock", func(t *testing.T){
		keys := []string{"key1", "key2", "key3"}

		for _, key := range keys {
			mm.Lock(key)
			// Lock should not block here
			mm.Unlock(key)
		}
	})

	t.Run("Concurrently lock and unlock mutexes", func(t *testing.T){
		keys := []string{"key1", "key2", "key3"}

		numGoroutines := 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Concurrently lock and unlock for each key
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for _, key := range keys {
					mm.Lock(key)
					mm.Unlock(key)
				}
			}()
		}
		wg.Wait()

	})

	t.Run("Lock and unlock nonexistent mutexes", func(t *testing.T){
		mm.Lock("non-existent-key")
		mm.Unlock("non-existent-key")

		_, ok := mm.mutex["non-existent-key"]
		require.True(t, ok)
	})
}
