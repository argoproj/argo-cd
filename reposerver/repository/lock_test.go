package repository

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	util "github.com/argoproj/argo-cd/v2/util/io"
)

// execute given action and return false if action have not completed within 1 second
func lockQuickly(action func() (io.Closer, error)) (io.Closer, bool) {
	done := make(chan io.Closer)
	go func() {
		closer, _ := action()
		done <- closer
	}()
	select {
	case <-time.After(1 * time.Second):
		return nil, false
	case closer := <-done:
		return closer, true
	}
}

func numberOfInits(initializedTimes *int) func() (io.Closer, error) {
	return func() (io.Closer, error) {
		*initializedTimes++
		return util.NopCloser, nil
	}
}

func TestLock_SameRevision(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	assert.Equal(t, 1, initializedTimes)

	util.Close(closer1)

	util.Close(closer2)
}

func TestLock_DifferentRevisions(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "2", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	util.Close(closer1)

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "2", true, init)
	})

	if !assert.True(t, done) {
		return
	}
}

func TestLock_NoConcurrentWithSameRevision(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", false, init)
	})

	if !assert.False(t, done) {
		return
	}

	util.Close(closer1)
}

func TestLock_FailedInitialization(t *testing.T) {
	lock := NewRepositoryLock()

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, func() (io.Closer, error) {
			return util.NopCloser, errors.New("failed")
		})
	})

	if !assert.True(t, done) {
		return
	}

	assert.Nil(t, closer1)

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, func() (io.Closer, error) {
			return util.NopCloser, nil
		})
	})

	if !assert.True(t, done) {
		return
	}

	util.Close(closer2)
}

func TestLock_SameRevisionFirstNotConcurrent(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	assert.Equal(t, 1, initializedTimes)

	util.Close(closer1)
}
