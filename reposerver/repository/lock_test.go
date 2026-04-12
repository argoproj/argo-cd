package repository

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
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

func numberOfInits(initializedTimes *int) func(_ bool) (io.Closer, error) {
	return func(_ bool) (io.Closer, error) {
		*initializedTimes++
		return utilio.NopCloser, nil
	}
}

func TestLock_SameRevision(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	assert.Equal(t, 1, initializedTimes)

	utilio.Close(closer1)

	utilio.Close(closer2)
}

func TestLock_DifferentRevisions(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "2", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	utilio.Close(closer1)

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "2", true, init)
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
		return lock.Lock(context.Background(), "myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", false, init)
	})

	if !assert.False(t, done) {
		return
	}

	utilio.Close(closer1)
}

func TestLock_FailedInitialization(t *testing.T) {
	lock := NewRepositoryLock()

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, func(_ bool) (io.Closer, error) {
			return utilio.NopCloser, errors.New("failed")
		})
	})

	if !assert.True(t, done) {
		return
	}

	assert.Nil(t, closer1)

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, func(_ bool) (io.Closer, error) {
			return utilio.NopCloser, nil
		})
	})

	if !assert.True(t, done) {
		return
	}

	utilio.Close(closer2)
}

func TestLock_WaiterForDifferentRevision_CannotBeUnblocked(t *testing.T) {
	lock := NewRepositoryLock()
	init := func(_ bool) (io.Closer, error) {
		return utilio.NopCloser, nil
	}

	// Acquire lock for revision "1"
	closer1, err := lock.Lock(context.Background(), "myRepo", "1", true, init)
	require.NoError(t, err)

	// Try to acquire lock for revision "2" with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err = lock.Lock(ctx, "myRepo", "2", true, init)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	utilio.Close(closer1)
}

func TestLock_ConvoyFormsUnderSequentialRevisions(t *testing.T) {
	lock := NewRepositoryLock()
	init := func(_ bool) (io.Closer, error) {
		return utilio.NopCloser, nil
	}

	// Acquire lock for revision "A" — simulates a long-running operation
	closerA, err := lock.Lock(context.Background(), "myRepo", "A", true, init)
	require.NoError(t, err)

	// Spawn 100 goroutines all waiting for revision "B" with short deadlines
	const n = 100
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, errs[idx] = lock.Lock(ctx, "myRepo", "B", true, init)
		}(i)
	}

	wg.Wait()

	// All goroutines should have exited via context cancellation
	for i, err := range errs {
		require.ErrorIs(t, err, context.DeadlineExceeded, "goroutine %d should have been cancelled", i)
	}

	utilio.Close(closerA)
}

func TestLock_SameRevisionFirstNotConcurrent(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	assert.Equal(t, 1, initializedTimes)

	utilio.Close(closer1)
}

func TestLock_CleanForNonConcurrent(t *testing.T) {
	lock := NewRepositoryLock()
	initClean := false
	init := func(clean bool) (io.Closer, error) {
		initClean = clean
		return utilio.NopCloser, nil
	}
	closer, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, init)
	})

	assert.True(t, done)
	// first time always clean because we cannot be sure about the state of repository
	assert.True(t, initClean)
	utilio.Close(closer)

	closer, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(context.Background(), "myRepo", "1", true, init)
	})

	assert.True(t, done)
	assert.False(t, initClean)
	utilio.Close(closer)
}
