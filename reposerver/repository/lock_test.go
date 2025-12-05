package repository

import (
	"context"
	"errors"
	"io"
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

func numberOfInits(initializedTimes *int) func() (io.Closer, error) {
	return func() (io.Closer, error) {
		*initializedTimes++
		return utilio.NopCloser, nil
	}
}

func TestLock_SameRevision(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", true, init)
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
		return lock.Lock(t.Context(), "myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "2", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	utilio.Close(closer1)

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "2", true, init)
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
		return lock.Lock(t.Context(), "myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", false, init)
	})

	if !assert.False(t, done) {
		return
	}

	utilio.Close(closer1)
}

func TestLock_FailedInitialization(t *testing.T) {
	lock := NewRepositoryLock()

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", true, func() (io.Closer, error) {
			return utilio.NopCloser, errors.New("failed")
		})
	})

	if !assert.True(t, done) {
		return
	}

	assert.Nil(t, closer1)

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", true, func() (io.Closer, error) {
			return utilio.NopCloser, nil
		})
	})

	if !assert.True(t, done) {
		return
	}

	utilio.Close(closer2)
}

func TestLock_SameRevisionFirstNotConcurrent(t *testing.T) {
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	assert.Equal(t, 1, initializedTimes)

	utilio.Close(closer1)
}

func TestLock_ContextCancelled(t *testing.T) {
	lock := NewRepositoryLock()
	init := numberOfInits(new(int))

	closer, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(t.Context(), "myRepo", "1", true, init)
	})
	if !assert.True(t, done) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error)
	go func() {
		_, err := lock.Lock(ctx, "myRepo", "2", true, init)
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(1 * time.Second):
		t.Fatal("Lock did not return after context cancellation")
	}

	utilio.Close(closer)
}

func TestLock_ContextAlreadyCancelled(t *testing.T) {
	lock := NewRepositoryLock()
	init := numberOfInits(new(int))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := lock.Lock(ctx, "myRepo", "1", true, init)
	require.ErrorIs(t, err, context.Canceled)
}
