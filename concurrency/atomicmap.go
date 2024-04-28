package concurrency

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type AtomicMap interface {
	Get(key string) (interface{}, error)
	GetOrCreate(key string) interface{}
	Delete(key string)
	OuterLock()
	OuterUnlock()
	OuterRLock()
	OuterRUnlock()
}

type AtomicMapBase struct {
	// This lock only protects the map itself, not the items in the map
	// The items are protected by their own atomic type
	mu sync.RWMutex
}

func (m *AtomicMapBase) OuterLock() {
	m.mu.Lock()
}

func (m *AtomicMapBase) OuterUnlock() {
	m.mu.Unlock()
}

// OuterRLock is usually needed when ranging over Items
func (m *AtomicMapBase) OuterRLock() {
	m.mu.RLock()
}

// OuterRUnlock is usually needed when ranging over Items
func (m *AtomicMapBase) OuterRUnlock() {
	m.mu.RUnlock()
}

type AtomicMapInt32 struct {
	AtomicMapBase
	Items map[string]*atomic.Int32
}

func NewAtomicMapInt32() *AtomicMapInt32 {
	return &AtomicMapInt32{
		Items: make(map[string]*atomic.Int32),
	}
}

func (m *AtomicMapInt32) Get(key string) (*atomic.Int32, error) {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		return nil, fmt.Errorf("key %s not found", key)
	}

	return item, nil
}

func (m *AtomicMapInt32) GetOrCreate(key string) *atomic.Int32 {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		m.OuterLock()
		// Double-check the key exists to avoid race condition
		if item, ok = m.Items[key]; !ok {
			item = &atomic.Int32{}
			m.Items[key] = item
		}
		m.OuterUnlock()
	}

	return item
}

func (m *AtomicMapInt32) Delete(key string) {
	m.OuterLock()
	defer m.OuterUnlock()

	delete(m.Items, key)
}

type AtomicMapUint32 struct {
	AtomicMapBase
	Items map[string]*atomic.Uint32
}

func NewAtomicMapUint32() *AtomicMapUint32 {
	return &AtomicMapUint32{
		Items: make(map[string]*atomic.Uint32),
	}
}

func (m *AtomicMapUint32) Get(key string) (*atomic.Uint32, error) {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		return nil, fmt.Errorf("key %s not found", key)
	}

	return item, nil
}

func (m *AtomicMapUint32) GetOrCreate(key string) *atomic.Uint32 {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		m.OuterLock()
		// Double-check the key exists to avoid race condition
		if item, ok = m.Items[key]; !ok {
			item = &atomic.Uint32{}
			m.Items[key] = item
		}
		m.OuterUnlock()
	}

	return item
}

func (m *AtomicMapUint32) Delete(key string) {
	m.OuterLock()
	defer m.OuterUnlock()

	delete(m.Items, key)
}

type AtomicMapInt64 struct {
	AtomicMapBase
	Items map[string]*atomic.Int64
}

func NewAtomicMapInt64() *AtomicMapInt64 {
	return &AtomicMapInt64{
		Items: make(map[string]*atomic.Int64),
	}
}

func (m *AtomicMapInt64) Get(key string) (*atomic.Int64, error) {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		return nil, fmt.Errorf("key %s not found", key)
	}

	return item, nil
}

func (m *AtomicMapInt64) GetOrCreate(key string) *atomic.Int64 {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		m.OuterLock()
		// Double-check the key exists to avoid race condition
		if item, ok = m.Items[key]; !ok {
			item = &atomic.Int64{}
			m.Items[key] = item
		}
		m.OuterUnlock()
	}

	return item
}

func (m *AtomicMapInt64) Delete(key string) {
	m.OuterLock()
	defer m.OuterUnlock()

	delete(m.Items, key)
}

type AtomicMapUint64 struct {
	AtomicMapBase
	Items map[string]*atomic.Uint64
}

func NewAtomicMapUint64() *AtomicMapUint64 {
	return &AtomicMapUint64{
		Items: make(map[string]*atomic.Uint64),
	}
}

func (m *AtomicMapUint64) Get(key string) (*atomic.Uint64, error) {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		return nil, fmt.Errorf("key %s not found", key)
	}

	return item, nil
}

func (m *AtomicMapUint64) GetOrCreate(key string) *atomic.Uint64 {
	m.OuterRLock()
	item, ok := m.Items[key]
	m.OuterRUnlock()

	if !ok {
		m.OuterLock()
		// Double-check the key exists to avoid race condition
		if item, ok = m.Items[key]; !ok {
			item = &atomic.Uint64{}
			m.Items[key] = item
		}
		m.OuterUnlock()
	}

	return item
}

func (m *AtomicMapUint64) Delete(key string) {
	m.OuterLock()
	defer m.OuterUnlock()

	delete(m.Items, key)
}
