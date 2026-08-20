package exec

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrShuttingDown is returned instead of starting a command once Shutdown has been called. Callers
// retry - checkoutRevision falls back to fetching and checking out FETCH_HEAD whenever a checkout
// fails - so without this a terminated command would just be replaced by a fresh one for the kubelet
// to SIGKILL, leaving behind the lock files Shutdown exists to avoid.
var ErrShuttingDown = errors.New("exec: shutting down")

var (
	shutdown     = make(chan struct{})
	shutdownOnce sync.Once
	inFlight     atomic.Int64
)

// Shutdown makes every running command terminate - each signals its own process group, so git gets to
// remove .git/index.lock - and stops new ones from starting. It reports how many commands were
// running. Nothing else reaches those processes: the kubelet signals only the container's init
// process, which forwards to this process alone, and each command sits in its own process group.
func Shutdown() int64 {
	running := inFlight.Load()
	shutdownOnce.Do(func() { close(shutdown) })
	return running
}
