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
func Wait(checkInterval, checkTimeout uint, f func() bool) bool {
	next := time.Tick(time.Duration(checkInterval) * time.Second)
	timeoutDuration := time.Duration(checkTimeout) * time.Second
	for {
		select {
		case <-next:
			// check
			if f() {
				// success
				return true
			}
		case <-time.After(timeoutDuration):
			// timeout
			break
		}
	}
	return false
}
