package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
)

type (
	testContextKey struct{}
	// fakeServerStream is a minimal grpc.ServerStream implementation
	// used for testing context replacement behavior.
	fakeServerStream struct {
		grpc.ServerStream
		ctx context.Context
	}
)

func (f *fakeServerStream) Context() context.Context {
	return f.ctx
}

func TestIsHealthMethod(t *testing.T) {
	tests := []struct {
		name       string
		fullMethod string
		expected   bool
	}{
		{
			name:       "health check",
			fullMethod: "/grpc.health.v1.Health/Check",
			expected:   true,
		},
		{
			name:       "health watch",
			fullMethod: "/grpc.health.v1.Health/Watch",
			expected:   true,
		},
		{
			name:       "non health method",
			fullMethod: "/argoproj.application.ApplicationService/Get",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHealthMethod(tt.fullMethod); got != tt.expected {
				t.Fatalf("IsHealthMethod(%q) = %v, want %v", tt.fullMethod, got, tt.expected)
			}
		})
	}
}

func TestServerStreamWithContext(t *testing.T) {
	baseCtx := context.Background()
	newCtx := context.WithValue(context.Background(), testContextKey{}, "value")

	baseStream := &fakeServerStream{
		ctx: baseCtx,
	}

	wrapped := &ServerStreamWithContext{
		ServerStream: baseStream,
		Ctx:          newCtx,
	}

	if wrapped.Context() != newCtx {
		t.Fatalf("expected context to be replaced")
	}
}
