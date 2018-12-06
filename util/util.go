package util

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// location to use for generating temporary files, such as the kubeconfig needed by kubectl
	TempDir string
)

func init() {
	fileInfo, err := os.Stat("/dev/shm")
	if err == nil && fileInfo.IsDir() {
		TempDir = "/dev/shm"
	}
}

type Closer interface {
	Close() error
}

// Close is a convenience function to close a object that has a Close() method, ignoring any errors
// Used to satisfy errcheck lint
func Close(c Closer) {
	_ = c.Close()
}

// DeleteFile is best effort deletion of a file
func DeleteFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	_ = os.Remove(path)
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

// RetryUntilSucceed keep retrying given action with specified timeout until action succeed or specified context is done.
func RetryUntilSucceed(action func() error, desc string, ctx context.Context, timeout time.Duration) {
	ctxCompleted := false
	go func() {
		select {
		case <-ctx.Done():
			ctxCompleted = true
		}
	}()
	for {
		log.Infof("Start %s", desc)
		err := action()
		if err == nil {
			log.Infof("Completed %s", desc)
			return
		}
		if err != nil {
			if ctxCompleted {
				log.Infof("Stop retrying %s", desc)
				return
			}
			log.Warnf("Failed to %s: %+v, retrying in %v", desc, err, timeout)
			time.Sleep(timeout)
		}

	}
}

func FirstNonEmpty(args ...string) string {
	for _, value := range args {
		if len(value) > 0 {
			return value
		}
	}
	return ""
}

func RunAllAsync(count int, action func(i int) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			message := fmt.Sprintf("Recovered from panic: %+v\n%s", r, debug.Stack())
			log.Error(message)
			err = errors.New(message)
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			err = action(index)
		}(i)
		if err != nil {
			break
		}
	}
	wg.Wait()
	return err
}
