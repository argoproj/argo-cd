package helm

import (
	"context"
	"sync"
)

type indexClientLock struct {
	mu      sync.Mutex
	entries map[string]*indexClientLockEntry
}

type indexClientLockEntry struct {
	available chan struct{}
	users     int
}

var indexClientLocks = indexClientLock{entries: make(map[string]*indexClientLockEntry)}

// Acquire serializes a scoped client's cache-read/fetch/publication without
// blocking cancellation or retaining a mutex for every retired credential view.
func (l *indexClientLock) acquire(ctx context.Context, key string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	entry := l.entries[key]
	if entry == nil {
		entry = &indexClientLockEntry{available: make(chan struct{}, 1)}
		entry.available <- struct{}{}
		l.entries[key] = entry
	}
	entry.users++
	l.mu.Unlock()
	forget := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		entry.users--
		if entry.users == 0 {
			delete(l.entries, key)
		}
	}
	select {
	case <-ctx.Done():
		forget()
		return nil, ctx.Err()
	case <-entry.available:
	}
	unlock := func() { entry.available <- struct{}{}; forget() }
	if err := ctx.Err(); err != nil {
		unlock()
		return nil, err
	}
	return unlock, nil
}
