package metrics

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/semaphore"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestEdgeCasesAndErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		teardown func()
		testFunc func(t *testing.T)
	}{
		{
			name: "lsRemoteParallelismLimitSemaphore is nil",
			testFunc: func(t *testing.T) {
				lsRemoteParallelismLimitSemaphore = nil
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil",
			setup: func() {
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
			},
			teardown: func() {
				lsRemoteParallelismLimitSemaphore = nil
			},
			testFunc: func(t *testing.T) {
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil and Acquire returns error",
			setup: func() {
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
			},
			teardown: func() {
				lsRemoteParallelismLimitSemaphore = nil
			},
			testFunc: func(t *testing.T) {
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.teardown != nil {
				defer tt.teardown()
			}
			tt.testFunc(t)
		})
	}
}

func TestSemaphoreFunctionality(t *testing.T) {
	os.Setenv("ARGOCD_GIT_LSREMOTE_PARALLELISM_LIMIT", "1")

	tests := []struct {
		name     string
		setup    func()
		teardown func()
		testFunc func(t *testing.T)
	}{
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil",
			setup: func() {
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
			},
			teardown: func() {
				lsRemoteParallelismLimitSemaphore = nil
			},
			testFunc: func(t *testing.T) {
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil and Acquire returns error",
			setup: func() {
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
			},
			teardown: func() {
				lsRemoteParallelismLimitSemaphore = nil
			},
			testFunc: func(t *testing.T) {
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.teardown != nil {
				defer tt.teardown()
			}
			tt.testFunc(t)
		})
	}
}
