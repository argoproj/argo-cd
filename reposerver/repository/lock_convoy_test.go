package repository

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

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
	assert.ErrorIs(t, err, context.DeadlineExceeded)

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

	// Spawn 10 goroutines all waiting for revision "B" with short deadlines
	const n = 10
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
		assert.ErrorIs(t, err, context.DeadlineExceeded, "goroutine %d should have been cancelled", i)
	}

	utilio.Close(closerA)
}
