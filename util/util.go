package util

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

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

// MakeSignature generates a cryptographically-secure pseudo-random token, based on a given number of random bytes, for signing purposes.
func MakeSignature(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		b = nil
	}
	// base64 encode it so signing key can be typed into validation utilities
	b = []byte(base64.StdEncoding.EncodeToString(b))
	return b, err
}
