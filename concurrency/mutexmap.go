package concurrency

import "sync"

type MutexMap struct {
	mu    sync.RWMutex             // outer lock
	mutex map[string]*sync.RWMutex // inner locks
}

func NewMutexMap() *MutexMap {
	return &MutexMap{
		mutex: make(map[string]*sync.RWMutex),
	}
}

func (mm *MutexMap) Lock(key string) {
	mm.OuterRLock()
	lock, ok := mm.mutex[key]
	mm.OuterRUnlock()

	if !ok {
		mm.OuterLock()
		lock, ok = mm.mutex[key]
		if !ok {
			mm.mutex[key] = &sync.RWMutex{}
			lock = mm.mutex[key]
		}
		mm.OuterUnlock()
	}
	lock.Lock()
}

func (mm *MutexMap) Unlock(key string) {
	mm.OuterLock()
	defer mm.OuterUnlock()

	if _, ok := mm.mutex[key]; ok {
		mm.mutex[key].Unlock()
	}
}

func (mm *MutexMap) RLock(key string) {
	mm.OuterRLock()
	lock, ok := mm.mutex[key]
	mm.OuterRUnlock()

	if !ok {
		mm.OuterLock()
		lock, ok = mm.mutex[key]
		if !ok {
			mm.mutex[key] = &sync.RWMutex{}
			lock = mm.mutex[key]
		}
		mm.OuterUnlock()
	}
	lock.RLock()
}

func (mm *MutexMap) RUnlock(key string) {
	mm.OuterLock()
	defer mm.OuterUnlock()

	if _, ok := mm.mutex[key]; ok {
		mm.mutex[key].RUnlock()
	}
}

// Add adds a new mutex to the map
// If the calling code already holds the outer lock, set lock parameter to false
func (mm *MutexMap) Add(key string) {
	mm.OuterLock()
	defer mm.OuterUnlock()

	if _, ok := mm.mutex[key]; !ok {
		mm.mutex[key] = &sync.RWMutex{}
	}
}

// Delete deletes a mutex from the map
// If the calling code already holds the outer lock, set lock parameter to false
func (mm *MutexMap) Delete(key string) {
	mm.OuterLock()
	defer mm.OuterUnlock()

	delete(mm.mutex, key)
}

func (mm *MutexMap) OuterLock() {
	mm.mu.Lock()
}

func (mm *MutexMap) OuterUnlock() {
	mm.mu.Unlock()
}

func (mm *MutexMap) OuterRLock() {
	mm.mu.RLock()
}

func (mm *MutexMap) OuterRUnlock() {
	mm.mu.RUnlock()
}
