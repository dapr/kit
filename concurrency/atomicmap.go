package concurrency

import (
	"sync/atomic"
	"sync"
)

type AtomicMap interface {
	Init(key string)
	Delete(key string)
	Get(key string) interface{}
}

type AtomicMapInt32 struct {
	mu    sync.RWMutex
	items map[string]atomic.Int32
}

func NewAtomicMapInt32() *AtomicMapInt32 {
	return &AtomicMapInt32{
		items: make(map[string]atomic.Int32),
	}
}

func (m *AtomicMapInt32) Init(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = atomic.Int32{}
}

func (m *AtomicMapInt32) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
}

func (m *AtomicMapInt32) Get(key string) atomic.Int32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.items[key]
}

type AtomicMapUint32 struct {
	mu    sync.RWMutex
	items map[string]atomic.Uint32
}

func NewAtomicMapUint32() *AtomicMapUint32 {
	return &AtomicMapUint32{
		items: make(map[string]atomic.Uint32),
	}
}

func (m *AtomicMapUint32) Init(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = atomic.Uint32{}
}

func (m *AtomicMapUint32) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
}

func (m *AtomicMapUint32) Get(key string) atomic.Uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.items[key]
}

type AtomicMapInt64 struct {
	mu    sync.RWMutex
	items map[string]atomic.Int64
}

// NewAtomicMapInt64 creates a new AtomicMapInt64 instance.
func NewAtomicMapInt64() *AtomicMapInt64 {
	return &AtomicMapInt64{
		items: make(map[string]atomic.Int64),
	}
}

// Add adds a new int64 value to the map with the given key.
func (m *AtomicMapInt64) Init(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = atomic.Int64{}
}

// Delete deletes the int64 value with the given key from the map.
func (m *AtomicMapInt64) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
}

// Get retrieves the int64 value associated with the given key from the map.
func (m *AtomicMapInt64) Get(key string) atomic.Int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.items[key]
}

type AtomicMapUint64 struct {
	mu    sync.RWMutex
	items map[string]atomic.Uint64
}

func NewAtomicMapUint64() *AtomicMapUint64 {
	return &AtomicMapUint64{
		items: make(map[string]atomic.Uint64),
	}
}

// Add adds a new int64 value to the map with the given key.
func (m *AtomicMapUint64) Init(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = atomic.Uint64{}
}

// Delete deletes the int64 value with the given key from the map.
func (m *AtomicMapUint64) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
}

// Get retrieves the int64 value associated with the given key from the map.
func (m *AtomicMapUint64) Get(key string) atomic.Uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.items[key]
}
