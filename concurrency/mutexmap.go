package concurrency

import "sync"

type MutexMap struct {
	mu    sync.RWMutex // outer lock
	mutex map[string]*sync.RWMutex // inner locks
}

func NewMutexMap() *MutexMap {
	return &MutexMap{
		mutex: make(map[string]*sync.RWMutex),
	}
}

func (mm *MutexMap) Lock(key string) {
	mm.mu.RLock()

	if mm.mutex[key] == nil {
		mm.mu.RUnlock()
		mm.mu.Lock()
		if mm.mutex[key] == nil {
			mm.mutex[key] = &sync.RWMutex{}
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

func (mm *MutexMap) RLock(key string) {
	mm.mu.RLock()

	if mm.mutex[key] == nil {
		mm.mu.RUnlock()
		mm.mu.Lock()
		if mm.mutex[key] == nil {
			mm.mutex[key] = &sync.RWMutex{}
		}
		mm.mutex[key].RLock()
		mm.mu.Unlock()
		return
	}
	mm.mutex[key].RLock()
	mm.mu.RUnlock()
}

func (mm *MutexMap) RUnlock(key string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.mutex[key] != nil {
		mm.mutex[key].RUnlock()
	}
}


func (mm *MutexMap) Add(key string, lock bool) {
	if lock {
		mm.mu.Lock()
		defer mm.mu.Unlock()
	}

	if mm.mutex[key] == nil {
		mm.mutex[key] = &sync.RWMutex{}
	}
}

func (mm *MutexMap) Delete(key string, lock bool) {
	if lock {
		mm.mu.Lock()
		defer mm.mu.Unlock()
	}

	if mm.mutex[key] != nil {
		delete(mm.mutex, key)
	}
}

func(mm *MutexMap) OuterLock(){
	mm.mu.Lock()
}

func(mm *MutexMap) OuterUnlock(){
	mm.mu.Unlock()
}

func(mm *MutexMap) OuterRLock(){
	mm.mu.RLock()
}

func(mm *MutexMap) OuterRUnlock(){
	mm.mu.RUnlock()
}
