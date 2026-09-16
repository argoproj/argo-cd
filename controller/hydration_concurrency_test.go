package controller

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/util/workqueue"

	hydratortypes "github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	"github.com/argoproj/argo-cd/v3/pkg/ratelimiter"
)

// TestNormalizeHydrationProcessors verifies that the hydration worker count is clamped to a safe minimum.
// The --hydration-processors CLI flag can be set to 0 or a negative value (the env default is clamped, but
// an explicit flag value is not), which would otherwise start zero workers and silently stall hydration.
// See https://github.com/argoproj/argo-cd/issues/27926.
func TestNormalizeHydrationProcessors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		configured int
		want       int
	}{
		{"zero clamps to one", 0, 1},
		{"negative clamps to one", -3, 1},
		{"one stays one", 1, 1},
		{"default preserved", 5, 5},
		{"large value preserved", 50, 50},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, normalizeHydrationProcessors(tc.configured))
		})
	}
}

func newTestHydrationQueue() workqueue.TypedRateLimitingInterface[hydratortypes.HydrationQueueKey] {
	return workqueue.NewTypedRateLimitingQueue(
		ratelimiter.NewCustomAppControllerRateLimiter[hydratortypes.HydrationQueueKey](ratelimiter.GetDefaultAppRateLimiterConfig()),
	)
}

// TestHydrationQueue_DistinctKeysProcessConcurrently asserts the core feature claim of #27926: when the
// hydration queue is drained by multiple workers, distinct hydration keys are processed concurrently, while
// no single key is ever processed by two workers at the same time. The worker loop mirrors the one started
// in ApplicationController.Run.
func TestHydrationQueue_DistinctKeysProcessConcurrently(t *testing.T) {
	t.Parallel()

	queue := newTestHydrationQueue()
	t.Cleanup(queue.ShutDown)

	const numKeys = 12
	const numWorkers = 4
	for i := range numKeys {
		queue.Add(hydratortypes.HydrationQueueKey{SourceRepoURL: fmt.Sprintf("https://example.com/repo-%d", i)})
	}

	var (
		mu             sync.Mutex
		inFlight       int
		maxConcurrent  int
		processed      int
		perKeyActive   = map[hydratortypes.HydrationQueueKey]bool{}
		sameKeyOverlap bool
	)

	processNext := func() bool {
		key, shutdown := queue.Get()
		if shutdown {
			return false
		}
		defer queue.Done(key)

		mu.Lock()
		if perKeyActive[key] {
			sameKeyOverlap = true
		}
		perKeyActive[key] = true
		inFlight++
		if inFlight > maxConcurrent {
			maxConcurrent = inFlight
		}
		mu.Unlock()

		// Hold the key briefly so that concurrent processing of distinct keys is observable.
		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		inFlight--
		perKeyActive[key] = false
		processed++
		allDone := processed == numKeys
		mu.Unlock()

		if allDone {
			queue.ShutDown()
		}
		return true
	}

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Go(func() {
			for processNext() {
			}
		})
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, numKeys, processed, "every distinct key should be processed exactly once")
	assert.False(t, sameKeyOverlap, "the same hydration key must never be processed by two workers at once")
	assert.GreaterOrEqual(t, maxConcurrent, 2, "distinct hydration keys should be processed concurrently with multiple workers")
}

// TestHydrationQueue_SameKeyNotProcessedConcurrently asserts the dedup guarantee the feature relies on: a
// hydration key that is re-enqueued while it is still in-flight is withheld until the in-flight processing
// calls Done, so the same key is never handed to two workers simultaneously. See #27926.
func TestHydrationQueue_SameKeyNotProcessedConcurrently(t *testing.T) {
	t.Parallel()

	queue := newTestHydrationQueue()
	t.Cleanup(queue.ShutDown)

	key := hydratortypes.HydrationQueueKey{SourceRepoURL: "https://example.com/repo", DestinationBranch: "env/dev"}
	queue.Add(key)

	started := make(chan struct{})
	release := make(chan struct{})
	secondPickup := make(chan hydratortypes.HydrationQueueKey, 1)

	// Worker 1 takes the key and holds it (does not call Done) until released.
	go func() {
		k, shutdown := queue.Get()
		if shutdown {
			return
		}
		close(started)
		<-release
		queue.Done(k)
	}()

	<-started
	// Re-enqueue the same key while it is still in-flight. The workqueue must not hand it to another
	// worker until the first one calls Done.
	queue.Add(key)

	// Worker 2 attempts to take work; it must block while the key is in-flight.
	go func() {
		k, shutdown := queue.Get()
		if !shutdown {
			secondPickup <- k
		}
	}()

	select {
	case <-secondPickup:
		t.Fatal("the same hydration key was delivered to a second worker while still in-flight")
	case <-time.After(150 * time.Millisecond):
		// Expected: the re-added key is withheld until the first worker calls Done.
	}

	// Release the first worker; the re-added key should now become available to worker 2.
	close(release)
	select {
	case got := <-secondPickup:
		assert.Equal(t, key, got)
		queue.Done(got)
	case <-time.After(2 * time.Second):
		t.Fatal("re-added key was not delivered after the first worker finished")
	}
}
