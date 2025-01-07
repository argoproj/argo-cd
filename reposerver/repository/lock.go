package repository

import (
	"fmt"
	"io"
	"sync"

	ioutil "github.com/argoproj/argo-cd/v2/util/io"
)

func NewRepositoryLock() *repositoryLock {
	return &repositoryLock{stateByKey: map[string]*repositoryState{}}
}

type repositoryLock struct {
	lock       sync.Mutex
	stateByKey map[string]*repositoryState
}

// Lock acquires lock unless lock is already acquired with the same commit and allowConcurrent is set to true
func (r *repositoryLock) Lock(path string, revision string, allowConcurrent bool, init func() (io.Closer, error)) (io.Closer, error) {
	r.lock.Lock()
	state, ok := r.stateByKey[path]
	if !ok {
		state = &repositoryState{cond: &sync.Cond{L: &sync.Mutex{}}}
		r.stateByKey[path] = state
	}
	r.lock.Unlock()

	closer := ioutil.NewCloser(func() error {
		state.cond.L.Lock()
		notify := false
		state.processCount--
		var err error
		if state.processCount == 0 {
			notify = true
			state.revision = ""
			err = state.initCloser.Close()
		}

		state.cond.L.Unlock()
		if notify {
			state.cond.Broadcast()
		}
		if err != nil {
			return fmt.Errorf("init closer failed: %w", err)
		}
		return nil
	})

	for {
		state.cond.L.Lock()
		if state.revision == "" {
			// no in progress operation for that repo. Go ahead.
			initCloser, err := init()
			if err != nil {
				state.cond.L.Unlock()
				return nil, fmt.Errorf("failed to initialize repository resources: %w", err)
			}
			state.initCloser = initCloser
			state.revision = revision
			state.processCount = 1
			state.allowConcurrent = allowConcurrent
			state.cond.L.Unlock()
			return closer, nil
		} else if state.revision == revision && state.allowConcurrent && allowConcurrent {
			// same revision already processing and concurrent processing allowed. Increment process count and go ahead.
			state.processCount++
			state.cond.L.Unlock()
			return closer, nil
		} else {
			state.cond.Wait()
			// wait when all in-flight processes of this revision complete and try again
			state.cond.L.Unlock()
		}
	}
}

type repositoryState struct {
	cond            *sync.Cond
	revision        string
	initCloser      io.Closer
	processCount    int
	allowConcurrent bool
}
