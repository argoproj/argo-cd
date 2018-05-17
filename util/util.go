package util

import "time"

type Closer interface {
	Close() error
}

// Close is a convenience function to close a object that has a Close() method, ignoring any errors
// Used to satisfy errcheck lint
func Close(c Closer) {
	_ = c.Close()
}

// Wait takes a check interval and timeout and waits for a function to return `true`.
// Wait will return `true` on success and `false` on timeout.
// The passed function, in turn, should return whether the desired state has been achieved yet.
func Wait(checkInterval, checkTimeout uint, check func() bool) bool {
	// Do an initial check before we start the timer
	if check() {
		return true
	}

	next := time.Tick(time.Duration(checkInterval) * time.Second)
	fail := time.After(time.Duration(checkTimeout) * time.Second)
	for {
		select {
		case <-next:
			if check() {
				return true
			}
		case <-fail:
			return false
		}
	}
}
