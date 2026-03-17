package repository

import (
	"context"
	"fmt"
	"io"
	"sync"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

func NewRepositoryLock() *repositoryLock {
	return &repositoryLock{stateByKey: map[string]*repositoryState{}}
}

type repositoryLock struct {
	lock       sync.Mutex
	stateByKey map[string]*repositoryState
}

// Lock acquires lock unless lock is already acquired with the same commit and allowConcurrent is set to true.
// The context allows callers to cancel waiting for the lock, preventing convoy deadlocks when
// goroutines for newer revisions pile up behind the current revision.
// The init callback receives `clean` parameter which indicates if repo state must be cleaned after running non-concurrent operation.
// The first init always runs with `clean` set to true because we cannot be sure about initial repo state.
func (r *repositoryLock) Lock(ctx context.Context, path string, revision string, allowConcurrent bool, init func(clean bool) (io.Closer, error)) (io.Closer, error) {
	r.lock.Lock()
	state, ok := r.stateByKey[path]
	if !ok {
		state = &repositoryState{broadcast: make(chan struct{})}
		r.stateByKey[path] = state
	}
	r.lock.Unlock()

	closer := utilio.NewCloser(func() error {
		state.mu.Lock()
		notify := false
		state.processCount--
		var err error
		if state.processCount == 0 {
			notify = true
			state.revision = ""
			err = state.initCloser.Close()
		}

		if notify {
			close(state.broadcast)
			state.broadcast = make(chan struct{})
		}
		state.mu.Unlock()
		if err != nil {
			return fmt.Errorf("init closer failed: %w", err)
		}
		return nil
	})

	for {
		state.mu.Lock()
		if state.revision == "" {
			// no in progress operation for that repo. Go ahead.
			initCloser, err := init(!state.allowConcurrent)
			if err != nil {
				state.mu.Unlock()
				return nil, fmt.Errorf("failed to initialize repository resources: %w", err)
			}
			state.initCloser = initCloser
			state.revision = revision
			state.processCount = 1
			state.allowConcurrent = allowConcurrent
			state.mu.Unlock()
			return closer, nil
		} else if state.revision == revision && state.allowConcurrent && allowConcurrent {
			// same revision already processing and concurrent processing allowed. Increment process count and go ahead.
			state.processCount++
			state.mu.Unlock()
			return closer, nil
		}
		ch := state.broadcast
		state.mu.Unlock()

		// wait when all in-flight processes of this revision complete and try again
		select {
		case <-ch:
			// broadcast received, retry
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type repositoryState struct {
	mu              sync.Mutex
	broadcast       chan struct{}
	revision        string
	initCloser      io.Closer
	processCount    int
	allowConcurrent bool
}
