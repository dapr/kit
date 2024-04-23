package concurrency

import (
	"sync/atomic"
	"sync"
)

type AtomicMap interface {
	Delete(key string)
	Get(key string) interface{}
	OuterLock()
	OuterUnlock()
	OuterRLock()
	OuterRUnlock()
}

type AtomicMapBase struct{
	mu sync.RWMutex
}

func(mm *AtomicMapBase) OuterLock(){
	mm.mu.Lock()
}

func(mm *AtomicMapBase) OuterUnlock(){
	mm.mu.Unlock()
}

func(mm *AtomicMapBase) OuterRLock(){
	mm.mu.RLock()
}

func(mm *AtomicMapBase) OuterRUnlock(){
	mm.mu.RUnlock()
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

func (m *AtomicMapInt32) Get(key string) *atomic.Int32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Items[key]; !ok {
		m.Items[key] = &atomic.Int32{}
	}


	return m.Items[key]
}

func (m *AtomicMapInt32) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

func (m *AtomicMapUint32) Get(key string) *atomic.Uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Items[key]; !ok {
		m.Items[key] = &atomic.Uint32{}
	}

	return m.Items[key]
}

func (m *AtomicMapUint32) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

func (m *AtomicMapInt64) Get(key string) *atomic.Int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Items[key]; !ok {
		m.Items[key] = &atomic.Int64{}
	}


	return m.Items[key]
}

func (m *AtomicMapInt64) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

func (m *AtomicMapUint64) Get(key string) *atomic.Uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Items[key]; !ok {
		m.Items[key] = &atomic.Uint64{}
	}

	return m.Items[key]
}

func (m *AtomicMapUint64) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Items, key)
}