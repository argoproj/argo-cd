package util

import (
	"time"
)

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
// The passed function, in turn, should pass `true` (or anything, really) to the channel when it's done.
// Pass `0` as the timeout to run infinitely until completion.
func Wait(timeout uint, f func(chan<- bool)) bool {
	done := make(chan bool)
	go f(done)

	// infinite
	if timeout == 0 {
		return <-done
	}

	timedOut := time.After(time.Duration(timeout) * time.Second)
	for {
		select {
		case <-done:
			return true
		case <-timedOut:
			return false
		}
	}
}
