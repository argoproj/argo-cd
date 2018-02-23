package util

import "sync"

// Allows to lock by string key
type KeyLock struct {
	giantLock sync.RWMutex
	locks     map[string]*sync.Mutex
}

// NewKeyLock creates new instance of KeyLock
func NewKeyLock() *KeyLock {
	return &KeyLock{
		giantLock: sync.RWMutex{},
		locks:     map[string]*sync.Mutex{},
	}
}

func (keyLock *KeyLock) getLock(key string) *sync.Mutex {
	keyLock.giantLock.RLock()
	if lock, ok := keyLock.locks[key]; ok {
		keyLock.giantLock.RUnlock()
		return lock
	}

	keyLock.giantLock.RUnlock()
	keyLock.giantLock.Lock()

	if lock, ok := keyLock.locks[key]; ok {
		keyLock.giantLock.Unlock()
		return lock
	}

	lock := &sync.Mutex{}
	keyLock.locks[key] = lock
	keyLock.giantLock.Unlock()
	return lock
}

// Lock blocks goroutine using key specific mutex
func (keyLock *KeyLock) Lock(key string) {
	keyLock.getLock(key).Lock()
}

// Unlock releases key specific mutex
func (keyLock *KeyLock) Unlock(key string) {
	keyLock.getLock(key).Unlock()
}
