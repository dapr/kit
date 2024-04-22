package concurrency

import "sync"

type MutexMap struct {
	mu    sync.RWMutex // outer lock
	mutex map[string]*sync.Mutex // inner locks
}

func NewMutexMap() *MutexMap {
	return &MutexMap{
		mutex: make(map[string]*sync.Mutex),
	}
}

func (mm *MutexMap) Lock(key string) {
	mm.mu.RLock()

	if mm.mutex[key] == nil {
		mm.mu.RUnlock()
		mm.mu.Lock()
		if mm.mutex[key] == nil {
			mm.mutex[key] = &sync.Mutex{}
		}
		mm.mutex[key].Lock()
		mm.mu.Unlock()
		return
	}
	mm.mutex[key].Lock()
	mm.mu.RUnlock()
}

func (mm *MutexMap) Unlock(key string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.mutex[key] != nil {
		mm.mutex[key].Unlock()
	}
}

func (mm *MutexMap) Add(key string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.mutex[key] == nil {
		mm.mutex[key] = &sync.Mutex{}
	}
}

func (mm *MutexMap) Delete(key string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.mutex[key] != nil {
		delete(mm.mutex, key)
	}
}
