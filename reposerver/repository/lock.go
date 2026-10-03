package repository

import (
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

// Lock acquires lock unless lock is already acquired with the same commit and allowConcurrent is set to true
// The init callback receives `clean` parameter which indicates if repo state must be cleaned after running non-concurrent operation.
// The first init always runs with `clean` set to true because we cannot be sure about initial repo state.
// When the revision being checked out differs from the last completed revision, the Helm dependency build
// marker files left by the previous revision are removed (see helmDepMarkers), so that `helm dependency build`
// is re-run for the new revision. A full clean is not performed in that case because it is too expensive for
// large repositories, see https://github.com/argoproj/argo-cd/issues/29856.
func (r *repositoryLock) Lock(path string, revision string, allowConcurrent bool, init func(clean bool) (io.Closer, error)) (io.Closer, error) {
	r.lock.Lock()
	state, ok := r.stateByKey[path]
	if !ok {
		state = &repositoryState{cond: &sync.Cond{L: &sync.Mutex{}}}
		r.stateByKey[path] = state
	}
	r.lock.Unlock()

	closer := utilio.NewCloser(func() error {
		state.cond.L.Lock()
		notify := false
		state.processCount--
		var err error
		if state.processCount == 0 {
			notify = true
			state.lastRevision = state.revision
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
			// Untracked files left by the previous revision (e.g. the Helm
			// dependency build marker files) must be removed so that they do
			// not affect the processing of the new revision.
			if state.lastRevision != revision {
				helmDepMarkers.removeAll(path)
			}
			initCloser, err := init(!state.allowConcurrent)
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
		}
		state.cond.Wait()
		// wait when all in-flight processes of this revision complete and try again
		state.cond.L.Unlock()
	}
}

type repositoryState struct {
	cond            *sync.Cond
	revision        string
	lastRevision    string
	initCloser      io.Closer
	processCount    int
	allowConcurrent bool
}
