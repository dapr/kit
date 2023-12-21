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
	"sync/atomic"
	"time"

	"github.com/alphadose/haxmap"
	kclock "k8s.io/utils/clock"
)

// Cache is an efficient cache with a TTL.
type Cache[V any] struct {
	m         *haxmap.Map[string, cacheEntry[V]]
	clock     kclock.WithTicker
	stopped   atomic.Bool
	runningCh chan struct{}
	stopCh    chan struct{}
	maxTTL    int64
}

// CacheOptions are options for NewCache.
type CacheOptions struct {
	// Initial size for the cache.
	// This is optional, and if empty will be left to the underlying library to decide.
	InitialSize int32

	// Interval to perform garbage collection.
	// This is optional, and defaults to 150s (2.5 minutes).
	CleanupInterval time.Duration

	// Maximum TTL value in seconds, if greater than 0
	MaxTTL int64

	// Internal clock property, used for testing
	clock kclock.WithTicker
}

// NewCache returns a new cache with a TTL.
func NewCache[V any](opts CacheOptions) *Cache[V] {
	var m *haxmap.Map[string, cacheEntry[V]]
	if opts.InitialSize > 0 {
		m = haxmap.New[string, cacheEntry[V]](uintptr(opts.InitialSize))
	} else {
		m = haxmap.New[string, cacheEntry[V]]()
	}

	if opts.CleanupInterval <= 0 {
		opts.CleanupInterval = 150 * time.Second
	}

	if opts.clock == nil {
		opts.clock = kclock.RealClock{}
	}

	c := &Cache[V]{
		m:      m,
		clock:  opts.clock,
		maxTTL: opts.MaxTTL,
		stopCh: make(chan struct{}),
	}
	c.startBackgroundCleanup(opts.CleanupInterval)
	return c
}

// Get returns an item from the cache.
// Items that have expired are not returned.
func (c *Cache[V]) Get(key string) (v V, ok bool) {
	val, ok := c.m.Get(key)
	if !ok || !val.exp.After(c.clock.Now()) {
		return v, false
	}
	return val.val, true
}

// Set an item in the cache.
func (c *Cache[V]) Set(key string, val V, ttl int64) {
	if ttl <= 0 {
		panic("invalid TTL: must be > 0")
	}

	if c.maxTTL > 0 && ttl > c.maxTTL {
		ttl = c.maxTTL
	}

	exp := c.clock.Now().Add(time.Duration(ttl) * time.Second)
	c.m.Set(key, cacheEntry[V]{
		val: val,
		exp: exp,
	})
}

// Cleanup removes all expired entries from the cache.
func (c *Cache[V]) Cleanup() {
	now := c.clock.Now()

	// Look for all expired keys and then remove them in bulk
	// This is more efficient than removing keys one-by-one
	// However, this could lead to a race condition where keys that are updated after ForEach ends are deleted nevertheless.
	// This is considered acceptable in this case as this is just a cache.
	keys := make([]string, 0, c.m.Len())
	c.m.ForEach(func(k string, v cacheEntry[V]) bool {
		if v.exp.Before(now) {
			keys = append(keys, k)
		}
		return true
	})

	c.m.Del(keys...)
}

// Reset removes all entries from the cache.
func (c *Cache[V]) Reset() {
	// Look for all keys and then remove them in bulk
	// This is more efficient than removing keys one-by-one
	// However, this could lead to a race condition where keys that are updated after ForEach ends are deleted nevertheless.
	// This is considered acceptable in this case as this is just a cache.
	keys := make([]string, 0, c.m.Len())
	c.m.ForEach(func(k string, v cacheEntry[V]) bool {
		keys = append(keys, k)
		return true
	})

	c.m.Del(keys...)
}

func (c *Cache[V]) startBackgroundCleanup(d time.Duration) {
	c.runningCh = make(chan struct{})
	go func() {
		defer close(c.runningCh)

		t := c.clock.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-c.stopCh:
				// Stop the background goroutine
				return
			case <-t.C():
				c.Cleanup()
			}
		}
	}()
}

// Stop the cache, stopping the background garbage collection process.
func (c *Cache[V]) Stop() {
	if c.stopped.CompareAndSwap(false, true) {
		close(c.stopCh)
	}
	<-c.runningCh
}

// Each item in the cache is stored in a cacheEntry, which includes the value as well as its expiration time.
type cacheEntry[V any] struct {
	val V
	exp time.Time
}
