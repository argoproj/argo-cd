package utils

import "sync"

// KeyLock allows for the locking of any key. Calls are blocking when trying to acquire an existing lock.
// defer keyLock.Lock("foo")()
type KeyLock struct {
	lockMap *sync.Map
}

// Lock locks a key and returns the unlock function
func (k *KeyLock) Lock(key string) func() {
	value, _ := k.lockMap.LoadOrStore(key, &sync.Mutex{})
	m := value.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// Delete removes a key from the store
func (k *KeyLock) Delete(key string) {
	k.lockMap.Delete(key)
}

func NewKeyLock() *KeyLock {
	var syncMap sync.Map
	return &KeyLock{lockMap: &syncMap}
}
