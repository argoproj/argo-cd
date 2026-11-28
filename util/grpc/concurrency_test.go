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

// Info values passed to the interceptors under test.
var (
	unaryInfo = &grpc.UnaryServerInfo{FullMethod: "/repository.RepoServerService/GenerateManifest"}
	// Server-streaming: limited, because these get retried on another replica.
	streamInfo = &grpc.StreamServerInfo{FullMethod: "/test.RepoServerService/ServerStream", IsServerStream: true}
	// Client-streaming: exempt, because it's never retried (rejecting it would be a hard failure).
	clientStreamInfo = &grpc.StreamServerInfo{FullMethod: "/repository.RepoServerService/GenerateManifestWithFiles", IsClientStream: true}
	unaryHealthInfo  = &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}
	streamHealthCfg  = &grpc.StreamServerInfo{FullMethod: "/grpc.health.v1.Health/Watch"}
)

// nopUnaryHandler is a gRPC unary handler that succeeds immediately.
var nopUnaryHandler grpc.UnaryHandler = func(_ context.Context, _ any) (any, error) {
	return nil, nil
}

func TestConcurrencyLimiterUnaryServerInterceptor_NoLimit(t *testing.T) {
	limiter := grpc_util.NewConcurrencyLimiter(0)
	interceptor := limiter.UnaryServerInterceptor()

	// Without a limit, requests should always succeed.
	for i := range 20 {
		_, err := interceptor(t.Context(), i, unaryInfo, nopUnaryHandler)
		require.NoError(t, err)
	}
	// Active count should be back at zero after all requests complete.
	assert.EqualValues(t, 0, limiter.ActiveRequests())
}

func TestConcurrencyLimiterUnaryServerInterceptor_WithinLimit(t *testing.T) {
	const limit = 5
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	interceptor := limiter.UnaryServerInterceptor()

	for i := range int(limit) {
		_, err := interceptor(t.Context(), i, unaryInfo, nopUnaryHandler)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 0, limiter.ActiveRequests())
}

func TestConcurrencyLimiterUnaryServerInterceptor_ExceedLimit(t *testing.T) {
	const limit = 2
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	interceptor := limiter.UnaryServerInterceptor()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var started sync.WaitGroup
	var released sync.WaitGroup

	// Start `limit` slow requests that hold the capacity.
	for range int(limit) {
		started.Add(1)
		released.Add(1)
		go func() {
			_, _ = interceptor(ctx, nil, unaryInfo, func(_ context.Context, _ any) (any, error) {
				started.Done()
				released.Wait() // wait until test tells us to release
				return nil, nil
			})
		}()
	}
	started.Wait() // all slots are now occupied

	// An extra request should be rejected with ResourceExhausted.
	_, err := interceptor(t.Context(), nil, unaryInfo, nopUnaryHandler)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	// Unblock the slow handlers.
	released.Add(-int(limit))
}

func TestConcurrencyLimiterUnaryServerInterceptor_ActiveReportedCorrectly(t *testing.T) {
	limiter := grpc_util.NewConcurrencyLimiter(0) // no limit, only tracking
	interceptor := limiter.UnaryServerInterceptor()

	var activeAtHandler atomic.Int64
	_, err := interceptor(t.Context(), nil, unaryInfo, func(_ context.Context, _ any) (any, error) {
		activeAtHandler.Store(limiter.ActiveRequests())
		return nil, nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, activeAtHandler.Load(), "active count should be 1 inside the handler")
	assert.EqualValues(t, 0, limiter.ActiveRequests(), "active count should return to 0 after the handler completes")
}

// The health-check RPC must bypass the limiter so that a saturated repo-server does not fail its
// own liveness probe (which self-dials and would otherwise be rejected with ResourceExhausted).
func TestConcurrencyLimiterUnaryServerInterceptor_HealthCheckExempt(t *testing.T) {
	const limit = 1
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	interceptor := limiter.UnaryServerInterceptor()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})

	// A regular request occupies the only slot.
	go func() {
		_, _ = interceptor(ctx, nil, unaryInfo, func(_ context.Context, _ any) (any, error) {
			close(started)
			<-release
			return nil, nil
		})
	}()
	<-started

	// Health check must succeed at capacity, without perturbing the active count.
	_, err := interceptor(t.Context(), nil, unaryHealthInfo, nopUnaryHandler)
	require.NoError(t, err)
	assert.EqualValues(t, 1, limiter.ActiveRequests(), "health check must not change the active count")

	close(release)
}

// --- Stream interceptor tests ---

func TestConcurrencyLimiterStreamServerInterceptor_ExceedLimit(t *testing.T) {
	const limit = 1
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	interceptor := limiter.StreamServerInterceptor()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var started sync.WaitGroup
	started.Add(1)
	var released sync.WaitGroup
	released.Add(1)

	go func() {
		_ = interceptor(nil, &mockServerStream{ctx: ctx}, streamInfo, func(_ any, _ grpc.ServerStream) error {
			started.Done()
			released.Wait()
			return nil
		})
	}()
	started.Wait() // slot is occupied

	// Second request should be rejected.
	err := interceptor(nil, &mockServerStream{ctx: t.Context()}, streamInfo, func(_ any, _ grpc.ServerStream) error {
		return nil
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	released.Done()
}

func TestConcurrencyLimiterStreamServerInterceptor_ActiveReportedCorrectly(t *testing.T) {
	limiter := grpc_util.NewConcurrencyLimiter(0)
	interceptor := limiter.StreamServerInterceptor()

	var activeAtHandler atomic.Int64
	err := interceptor(nil, &mockServerStream{ctx: t.Context()}, streamInfo, func(_ any, _ grpc.ServerStream) error {
		activeAtHandler.Store(limiter.ActiveRequests())
		return nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, activeAtHandler.Load())
	assert.EqualValues(t, 0, limiter.ActiveRequests())
}

// The streaming health watch must also bypass the limiter.
func TestConcurrencyLimiterStreamServerInterceptor_HealthCheckExempt(t *testing.T) {
	const limit = 1
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	interceptor := limiter.StreamServerInterceptor()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_ = interceptor(nil, &mockServerStream{ctx: ctx}, streamInfo, func(_ any, _ grpc.ServerStream) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	err := interceptor(nil, &mockServerStream{ctx: t.Context()}, streamHealthCfg, func(_ any, _ grpc.ServerStream) error {
		return nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, limiter.ActiveRequests(), "health watch must not change the active count")

	close(release)
}

// Client-streaming RPCs bypass the limiter: they're never retried, so rejecting them would be a hard
// failure rather than backpressure.
func TestConcurrencyLimiterStreamServerInterceptor_ClientStreamExempt(t *testing.T) {
	const limit = 1
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	interceptor := limiter.StreamServerInterceptor()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})

	// A server-streaming request occupies the only slot.
	go func() {
		_ = interceptor(nil, &mockServerStream{ctx: ctx}, streamInfo, func(_ any, _ grpc.ServerStream) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	// Client-streaming request must succeed at capacity, without perturbing the active count.
	err := interceptor(nil, &mockServerStream{ctx: t.Context()}, clientStreamInfo, func(_ any, _ grpc.ServerStream) error {
		return nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, limiter.ActiveRequests(), "client-streaming RPC must not change the active count")

	close(release)
}

// The unary and stream interceptors must share a single counter, so a mixed workload is limited by
// total gRPC concurrency rather than a separate budget per RPC kind.
func TestConcurrencyLimiter_UnaryAndStreamShareCounter(t *testing.T) {
	const limit = 2
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	unary := limiter.UnaryServerInterceptor()
	stream := limiter.StreamServerInterceptor()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{}, 2)
	release := make(chan struct{})

	// One unary and one stream request together fill the limit of 2.
	go func() {
		_, _ = unary(ctx, nil, unaryInfo, func(_ context.Context, _ any) (any, error) {
			started <- struct{}{}
			<-release
			return nil, nil
		})
	}()
	go func() {
		_ = stream(nil, &mockServerStream{ctx: ctx}, streamInfo, func(_ any, _ grpc.ServerStream) error {
			started <- struct{}{}
			<-release
			return nil
		})
	}()
	<-started
	<-started
	assert.EqualValues(t, 2, limiter.ActiveRequests())

	// A third request of either kind must now be rejected — the counter is shared.
	_, err := unary(t.Context(), nil, unaryInfo, nopUnaryHandler)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	err = stream(nil, &mockServerStream{ctx: t.Context()}, streamInfo, func(_ any, _ grpc.ServerStream) error {
		return nil
	})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	close(release)
}

// ActiveRequests is what the metric gauge reads on each scrape, so it must reflect the in-flight
// request while a handler runs and return to zero once all work completes.
func TestConcurrencyLimiter_ActiveRequestsTracksMetric(t *testing.T) {
	limiter := grpc_util.NewConcurrencyLimiter(0)
	interceptor := limiter.UnaryServerInterceptor()

	var duringHandler int64
	_, err := interceptor(t.Context(), nil, unaryInfo, func(_ context.Context, _ any) (any, error) {
		duringHandler = limiter.ActiveRequests()
		return nil, nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, duringHandler, "ActiveRequests must reflect the in-flight request")
	assert.EqualValues(t, 0, limiter.ActiveRequests(), "ActiveRequests must return to zero after completion")
}

// mockServerStream implements grpc.ServerStream minimally.
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) Context() context.Context     { return m.ctx }
func (m *mockServerStream) SendMsg(any) error            { return nil }
func (m *mockServerStream) RecvMsg(any) error            { return nil }

// Ensure the mock satisfies the interface at compile time.
var _ grpc.ServerStream = (*mockServerStream)(nil)

// --- Regression: a rejected request must not leave the active count inflated. ---
func TestConcurrencyLimiterUnaryServerInterceptor_RejectedRequestNotCounted(t *testing.T) {
	const limit = 1
	limiter := grpc_util.NewConcurrencyLimiter(limit)
	interceptor := limiter.UnaryServerInterceptor()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_, _ = interceptor(ctx, nil, unaryInfo, func(_ context.Context, _ any) (any, error) {
			close(started)
			<-release
			return nil, nil
		})
	}()
	<-started // the first request holds the slot

	// Second request is rejected – active count should still be 1 (held by first).
	_, err := interceptor(t.Context(), nil, unaryInfo, nopUnaryHandler)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.EqualValues(t, 1, limiter.ActiveRequests(), "active count should be 1 (held by first request)")

	close(release) // unblock first request
}
