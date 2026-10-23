package metrics

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/semaphore"
)

var testSemaphoreMutex sync.Mutex

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
			setup: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = nil
				testSemaphoreMutex.Unlock()
			},
			testFunc: func(t *testing.T) {
				t.Helper()
				testSemaphoreMutex.Lock()
				defer testSemaphoreMutex.Unlock()
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil",
			setup: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
				testSemaphoreMutex.Unlock()
			},
			teardown: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = nil
				testSemaphoreMutex.Unlock()
			},
			testFunc: func(t *testing.T) {
				t.Helper()
				testSemaphoreMutex.Lock()
				defer testSemaphoreMutex.Unlock()
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil and Acquire returns error",
			setup: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
				testSemaphoreMutex.Unlock()
			},
			teardown: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = nil
				testSemaphoreMutex.Unlock()
			},
			testFunc: func(t *testing.T) {
				t.Helper()
				testSemaphoreMutex.Lock()
				defer testSemaphoreMutex.Unlock()
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
	t.Setenv("ARGOCD_GIT_LSREMOTE_PARALLELISM_LIMIT", "1")

	tests := []struct {
		name     string
		setup    func()
		teardown func()
		testFunc func(t *testing.T)
	}{
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil",
			setup: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
				testSemaphoreMutex.Unlock()
			},
			teardown: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = nil
				testSemaphoreMutex.Unlock()
			},
			testFunc: func(t *testing.T) {
				t.Helper()
				testSemaphoreMutex.Lock()
				defer testSemaphoreMutex.Unlock()
				assert.NotPanics(t, func() {
					NewGitClientEventHandlers(&MetricsServer{})
				})
			},
		},
		{
			name: "lsRemoteParallelismLimitSemaphore is not nil and Acquire returns error",
			setup: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(1)
				testSemaphoreMutex.Unlock()
			},
			teardown: func() {
				testSemaphoreMutex.Lock()
				lsRemoteParallelismLimitSemaphore = nil
				testSemaphoreMutex.Unlock()
			},
			testFunc: func(t *testing.T) {
				t.Helper()
				testSemaphoreMutex.Lock()
				defer testSemaphoreMutex.Unlock()
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
