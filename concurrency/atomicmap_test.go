package concurrency

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicMapInt32_New_Get_Delete(t *testing.T) {
	m := NewAtomicMapStringInt32()
	require.NotNil(t, m)
	require.NotNil(t, m.items)
	require.Empty(t, m.items)

	t.Run("basic operations", func(t *testing.T) {
		key := "key1"
		value := int32(10)

		// Initially, the key should not exist
		_, ok := m.Get(key)
		require.False(t, ok)

		// Add a value and check it
		m.GetOrCreate(key, 0).Store(value)
		result, ok := m.Get(key)
		require.True(t, ok)
		assert.Equal(t, value, result.Load())

		// Delete the key and check it no longer exists
		m.Delete(key)
		_, ok = m.Get(key)
		require.False(t, ok)
	})

	t.Run("concurrent access multiple keys", func(t *testing.T) {
		var wg sync.WaitGroup
		keys := []string{"key1", "key2", "key3"}
		iterations := 100

		wg.Add(len(keys) * 2)
		for _, key := range keys {
			go func(k string) {
				defer wg.Done()
				for i := 0; i < iterations; i++ {
					m.GetOrCreate(k, 0).Add(1)
				}
			}(key)
			go func(k string) {
				defer wg.Done()
				for i := 0; i < iterations; i++ {
					m.GetOrCreate(k, 0).Add(-1)
				}
			}(key)
		}
		wg.Wait()

		for _, key := range keys {
			val, ok := m.Get(key)
			require.True(t, ok)
			require.Equal(t, int32(0), val.Load())
		}
	})
}
