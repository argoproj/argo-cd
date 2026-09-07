package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAcquireParallelismSlot_NoSemaphore(t *testing.T) {
	// With no semaphore configured the call must succeed and return a no-op release func.
	release, err := acquireParallelismSlot(t.Context(), operationSettings{})
	require.NoError(t, err)
	require.NotNil(t, release)
	assert.NotPanics(t, release)
}

func TestAcquireParallelismSlot_Blocking(t *testing.T) {
	sem := semaphore.NewWeighted(1)
	settings := operationSettings{sem: sem, semLimit: 1, semFailFast: false}

	// First acquisition succeeds.
	release, err := acquireParallelismSlot(t.Context(), settings)
	require.NoError(t, err)

	// In blocking mode, a second acquisition must wait rather than fail. Prove it blocks by
	// cancelling the context and expecting the context error (not ResourceExhausted).
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = acquireParallelismSlot(ctx, settings)
	require.Error(t, err)
	assert.NotEqual(t, codes.ResourceExhausted, status.Code(err), "blocking mode must not fail-fast with ResourceExhausted")

	// Releasing the first slot lets a subsequent acquisition succeed.
	release()
	release2, err := acquireParallelismSlot(t.Context(), settings)
	require.NoError(t, err)
	release2()
}

func TestAcquireParallelismSlot_FailFast(t *testing.T) {
	sem := semaphore.NewWeighted(2)
	settings := operationSettings{sem: sem, semLimit: 2, semFailFast: true}

	// Fill all slots.
	release1, err := acquireParallelismSlot(t.Context(), settings)
	require.NoError(t, err)
	release2, err := acquireParallelismSlot(t.Context(), settings)
	require.NoError(t, err)

	// The next request must be rejected immediately with ResourceExhausted.
	start := time.Now()
	_, err = acquireParallelismSlot(t.Context(), settings)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Less(t, time.Since(start), time.Second, "fail-fast must not block")

	// After releasing a slot, a new request succeeds again.
	release1()
	release3, err := acquireParallelismSlot(t.Context(), settings)
	require.NoError(t, err)

	release2()
	release3()
}
