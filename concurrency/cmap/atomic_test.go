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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicInt32_New_Get_Delete(t *testing.T) {
	m := NewAtomic[string, int32]().(*atomicMap[string, int32])

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
				for range iterations {
					m.GetOrCreate(k, 0).Add(1)
				}
			}(key)
			go func(k string) {
				defer wg.Done()
				for range iterations {
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
