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

// Info values passed to the interceptors under test. A regular (non-health) method is used unless a
// test specifically exercises the health-check exemption.
var (
	unaryInfo       = &grpc.UnaryServerInfo{FullMethod: "/repository.RepoServerService/GenerateManifest"}
	streamInfo      = &grpc.StreamServerInfo{FullMethod: "/repository.RepoServerService/GenerateManifestWithFiles"}
	unaryHealthInfo = &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}
	streamHealthCfg = &grpc.StreamServerInfo{FullMethod: "/grpc.health.v1.Health/Watch"}
)

// gaugeRecorder captures the most recent active count reported by the interceptor, mimicking a
// metric gauge driven by the limiter's own counter.
type gaugeRecorder struct {
	active atomic.Int64
}

func (g *gaugeRecorder) set(active int64) { g.active.Store(active) }

// nopUnaryHandler is a gRPC unary handler that succeeds immediately.
var nopUnaryHandler grpc.UnaryHandler = func(_ context.Context, _ any) (any, error) {
	return nil, nil
}

func TestConcurrencyLimiterUnaryServerInterceptor_NoLimit(t *testing.T) {
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(0, rec.set)

	// Without a limit, requests should always succeed.
	for i := range 20 {
		_, err := interceptor(t.Context(), i, unaryInfo, nopUnaryHandler)
		require.NoError(t, err)
	}
	// Reported active count should be back at zero after all requests complete.
	assert.EqualValues(t, 0, rec.active.Load())
}

func TestConcurrencyLimiterUnaryServerInterceptor_WithinLimit(t *testing.T) {
	const limit = 5
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(limit, rec.set)

	for i := range int(limit) {
		_, err := interceptor(t.Context(), i, unaryInfo, nopUnaryHandler)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 0, rec.active.Load())
}

func TestConcurrencyLimiterUnaryServerInterceptor_ExceedLimit(t *testing.T) {
	const limit = 2
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(limit, rec.set)

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
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(0, rec.set) // no limit, only tracking

	var activeAtHandler atomic.Int64
	_, err := interceptor(t.Context(), nil, unaryInfo, func(_ context.Context, _ any) (any, error) {
		activeAtHandler.Store(rec.active.Load())
		return nil, nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, activeAtHandler.Load(), "reported active count should be 1 inside the handler")
	assert.EqualValues(t, 0, rec.active.Load(), "reported active count should return to 0 after the handler completes")
}

func TestConcurrencyLimiterUnaryServerInterceptor_NilReporter(t *testing.T) {
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(5, nil)
	// Should not panic when the reporter is nil.
	_, err := interceptor(t.Context(), nil, unaryInfo, nopUnaryHandler)
	require.NoError(t, err)
}

// The health-check RPC must bypass the limiter so that a saturated repo-server does not fail its
// own liveness probe (which self-dials and would otherwise be rejected with ResourceExhausted).
func TestConcurrencyLimiterUnaryServerInterceptor_HealthCheckExempt(t *testing.T) {
	const limit = 1
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(limit, rec.set)

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

	// The health check must still succeed even though the limiter is at capacity, and it must not
	// perturb the reported active count.
	_, err := interceptor(t.Context(), nil, unaryHealthInfo, nopUnaryHandler)
	require.NoError(t, err)
	assert.EqualValues(t, 1, rec.active.Load(), "health check must not change the active count")

	close(release)
}

// --- Stream interceptor tests ---

func TestConcurrencyLimiterStreamServerInterceptor_ExceedLimit(t *testing.T) {
	const limit = 1
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterStreamServerInterceptor(limit, rec.set)

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
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterStreamServerInterceptor(0, rec.set)

	var activeAtHandler atomic.Int64
	err := interceptor(nil, &mockServerStream{ctx: t.Context()}, streamInfo, func(_ any, _ grpc.ServerStream) error {
		activeAtHandler.Store(rec.active.Load())
		return nil
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, activeAtHandler.Load())
	assert.EqualValues(t, 0, rec.active.Load())
}

// The streaming health watch must also bypass the limiter.
func TestConcurrencyLimiterStreamServerInterceptor_HealthCheckExempt(t *testing.T) {
	const limit = 1
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterStreamServerInterceptor(limit, rec.set)

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
	assert.EqualValues(t, 1, rec.active.Load(), "health watch must not change the active count")

	close(release)
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

// --- Regression: a rejected request must not leave the reported active count inflated, and the
// limiter's counter must never drift from the reported gauge value. ---
func TestConcurrencyLimiterUnaryServerInterceptor_RejectedRequestNotCounted(t *testing.T) {
	const limit = 1
	rec := &gaugeRecorder{}
	interceptor := grpc_util.ConcurrencyLimiterUnaryServerInterceptor(limit, rec.set)

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

	// Second request is rejected – reported active count should still be 1 (held by first).
	_, err := interceptor(t.Context(), nil, unaryInfo, nopUnaryHandler)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.EqualValues(t, 1, rec.active.Load(), "active count should be 1 (held by first request)")

	close(release) // unblock first request
}
