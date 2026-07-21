package grpc_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
)

// mockTracker is a simple ActiveRequestsTracker for testing.
type mockTracker struct {
	active atomic.Int64
}

func (m *mockTracker) IncActiveGRPCRequests() { m.active.Add(1) }
func (m *mockTracker) DecActiveGRPCRequests() { m.active.Add(-1) }

// nopUnaryHandler is a gRPC unary handler that succeeds immediately.
var nopUnaryHandler grpc.UnaryHandler = func(ctx context.Context, req any) (any, error) {
	return nil, nil
}

// slowUnaryHandler is a gRPC unary handler that blocks until ctx is done.
var slowUnaryHandler grpc.UnaryHandler = func(ctx context.Context, req any) (any, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestConcurrencyLimiterUnaryServerInterceptor_NoLimit(t *testing.T) {
	tracker := &mockTracker{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(0, tracker)

	// Without a limit, requests should always succeed.
	for i := range 20 {
		_, err := interceptor(t.Context(), i, nil, nopUnaryHandler)
		require.NoError(t, err)
	}
	// Tracker should be back at zero after all requests complete.
	assert.EqualValues(t, 0, tracker.active.Load())
}

func TestConcurrencyLimiterUnaryServerInterceptor_WithinLimit(t *testing.T) {
	const limit = 5
	tracker := &mockTracker{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(limit, tracker)

	for i := range int(limit) {
		_, err := interceptor(t.Context(), i, nil, nopUnaryHandler)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 0, tracker.active.Load())
}

func TestConcurrencyLimiterUnaryServerInterceptor_ExceedLimit(t *testing.T) {
	const limit = 2
	tracker := &mockTracker{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(limit, tracker)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var started sync.WaitGroup
	var released sync.WaitGroup

	// Start `limit` slow requests that hold the capacity.
	for range int(limit) {
		started.Add(1)
		released.Add(1)
		go func() {
			_, _ = interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
				started.Done()
				released.Wait() // wait until test tells us to release
				return nil, nil
			})
		}()
	}
	started.Wait() // all slots are now occupied

	// An extra request should be rejected with ResourceExhausted.
	_, err := interceptor(t.Context(), nil, nil, nopUnaryHandler)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	// Unblock the slow handlers.
	released.Add(-int(limit))
}

func TestConcurrencyLimiterUnaryServerInterceptor_TrackerCalledCorrectly(t *testing.T) {
	tracker := &mockTracker{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(0, tracker) // no limit, only tracking

	var activeAtHandler atomic.Int64
	_, err := interceptor(t.Context(), nil, nil, func(_ context.Context, _ any) (any, error) {
		activeAtHandler.Store(tracker.active.Load())
		return nil, nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, activeAtHandler.Load(), "tracker should show 1 active request inside the handler")
	assert.EqualValues(t, 0, tracker.active.Load(), "tracker should return to 0 after the handler completes")
}

func TestConcurrencyLimiterUnaryServerInterceptor_NilTracker(t *testing.T) {
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(5, nil)
	// Should not panic when tracker is nil.
	_, err := interceptor(t.Context(), nil, nil, nopUnaryHandler)
	require.NoError(t, err)
}

// --- Stream interceptor tests ---

func TestConcurrencyLimiterStreamServerInterceptor_ExceedLimit(t *testing.T) {
	const limit = 1
	tracker := &mockTracker{}
	interceptor := grpc_util.ConcurrencyLimiterStreamServerInterceptor(limit, tracker)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var started sync.WaitGroup
	started.Add(1)
	var released sync.WaitGroup
	released.Add(1)

	go func() {
		_ = interceptor(nil, &mockServerStream{ctx: ctx}, nil, func(_ any, _ grpc.ServerStream) error {
			started.Done()
			released.Wait()
			return nil
		})
	}()
	started.Wait() // slot is occupied

	// Second request should be rejected.
	err := interceptor(nil, &mockServerStream{ctx: t.Context()}, nil, func(_ any, _ grpc.ServerStream) error {
		return nil
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	released.Done()
}

func TestConcurrencyLimiterStreamServerInterceptor_TrackerCalledCorrectly(t *testing.T) {
	tracker := &mockTracker{}
	interceptor := grpc_util.ConcurrencyLimiterStreamServerInterceptor(0, tracker)

	var activeAtHandler atomic.Int64
	err := interceptor(nil, &mockServerStream{ctx: t.Context()}, nil, func(_ any, _ grpc.ServerStream) error {
		activeAtHandler.Store(tracker.active.Load())
		return nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, activeAtHandler.Load())
	assert.EqualValues(t, 0, tracker.active.Load())
}

// mockServerStream implements grpc.ServerStream minimally.
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) Context() context.Context { return m.ctx }
func (m *mockServerStream) SendMsg(any) error        { return nil }
func (m *mockServerStream) RecvMsg(any) error        { return nil }

// Ensure the mock satisfies the interface at compile time.
var _ grpc.ServerStream = (*mockServerStream)(nil)

// --- Regression: rejected requests should not count against active gauge ---
func TestConcurrencyLimiterUnaryServerInterceptor_RejectedRequestNotCounted(t *testing.T) {
	const limit = 1
	tracker := &mockTracker{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(limit, tracker)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_, _ = interceptor(ctx, nil, nil, func(_ context.Context, _ any) (any, error) {
			close(started)
			<-release
			return nil, nil
		})
	}()
	<-started // the first request holds the slot

	// Second request is rejected – tracker should still be 1 (held by first).
	_, err := interceptor(t.Context(), nil, nil, nopUnaryHandler)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.EqualValues(t, 1, tracker.active.Load(), "active count should be 1 (held by first request)")

	close(release) // unblock first request
}
