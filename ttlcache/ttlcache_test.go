/*
Copyright 2023 The Dapr Authors
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

package ttlcache

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"
)

func TestCache(t *testing.T) {
	clock := &clocktesting.FakeClock{}
	clock.SetTime(time.Now())

	cache := NewCache[string](CacheOptions{
		InitialSize:     10,
		CleanupInterval: 20 * time.Second,
		MaxTTL:          15,
		clock:           clock,
	})
	defer cache.Stop()

	// Set values in the cache
	cache.Set("key1", "val1", 2)
	cache.Set("key2", "val2", 5)
	cache.Set("key3", "val3", 30) // Max TTL is 15s
	cache.Set("key4", "val4", 5)

	// Retrieve values
	for i := range 16 {
		v, ok := cache.Get("key1")
		if i < 2 {
			require.True(t, ok)
			require.Equal(t, "val1", v)
		} else {
			require.False(t, ok)
		}

		v, ok = cache.Get("key2")
		if i < 5 {
			require.True(t, ok)
			require.Equal(t, "val2", v)
		} else {
			require.False(t, ok)
		}

		v, ok = cache.Get("key3")
		if i < 15 {
			require.True(t, ok)
			require.Equal(t, "val3", v)
		} else {
			require.False(t, ok)
		}

		v, ok = cache.Get("key4")
		if i < 1 {
			require.True(t, ok)
			require.Equal(t, "val4", v)

			// Delete from the cache
			cache.Delete("key4")
		} else {
			require.False(t, ok)
		}

		// Advance the clock
		clock.Step(time.Second)
		runtime.Gosched()
		time.Sleep(20 * time.Millisecond)
	}

	// Values should still be in the cache as they haven't been cleaned up yet
	require.EqualValues(t, 3, cache.m.Len())

	// Advance the clock a bit more to make sure the cleanup runs
	clock.Step(5 * time.Second)

	runtime.Gosched()
	time.Sleep(20 * time.Millisecond)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		if !assert.EqualValues(c, 0, cache.m.Len()) {
			runtime.Gosched()
		}
	}, time.Second, 50*time.Millisecond)
}
