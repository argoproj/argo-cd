// Package reaper reaps orphaned child processes that are reparented to the
// current process when it runs as PID 1, e.g. when argocd-cmp-server is the
// entrypoint of a Config Management Plugin sidecar container.
//
// In a container, PID 1 inherits init's duty of calling wait(2) on any process
// that gets reparented to it. Plugin tooling (kustomize, helm, git, ...) often
// forks children that outlive their direct parent; once reparented to PID 1
// and exited, they stay zombies forever unless PID 1 reaps them. Each zombie
// holds a slot in the container's pids cgroup, so they eventually exhaust it
// and every fork/exec in the container fails with EAGAIN.
//
// The reaper deliberately does NOT call wait4(-1, ...): that would race with
// os/exec's own wait on directly-managed plugin processes. In Go, both
// (*os.Process).pidWait and pidfdWait return an ECHILD error (not success)
// when another thread has already consumed the child's exit status, so a
// blanket reaper would randomly turn successful plugin commands into
// "waitid: no child processes" failures. Instead, callers that manage their
// own children register them via StartTracked/Untrack, and the reaper only
// ever waits on zombie children it finds in /proc that are not tracked.
package reaper

import (
	"os/exec"
	"sync"
)

var (
	// spawnMu closes the window between fork (cmd.Start) and registration of
	// the child pid in tracked. Spawners hold the read lock across
	// Start+track; the reaper holds the write lock while it scans for and
	// reaps untracked zombies, so it can never observe (and steal) a
	// directly-managed child that has not been registered yet.
	spawnMu sync.RWMutex

	trackedMu sync.Mutex
	tracked   = make(map[int]uint64)
	trackSeq  uint64
)

// StartTracked starts cmd and registers its pid as directly managed, so the
// PID-1 zombie reaper never consumes the exit status that cmd.Wait expects.
// It returns a registration token: callers must call Untrack with
// cmd.Process.Pid and that token after cmd.Wait returns.
func StartTracked(cmd *exec.Cmd) (uint64, error) {
	spawnMu.RLock()
	defer spawnMu.RUnlock()
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return track(cmd.Process.Pid), nil
}

// Untrack removes a pid registered by StartTracked, but only while it still
// holds the same registration token. If the kernel has reused the pid for a
// newer tracked command in the meantime, the newer registration replaced the
// token and a stale Untrack is a no-op — otherwise it would expose the newer
// command's child to the reaper and fail its cmd.Wait with ECHILD. Call it
// only after the process has been waited on (cmd.Wait returned).
func Untrack(pid int, token uint64) {
	trackedMu.Lock()
	defer trackedMu.Unlock()
	if tracked[pid] == token {
		delete(tracked, pid)
	}
}

func track(pid int) uint64 {
	trackedMu.Lock()
	defer trackedMu.Unlock()
	trackSeq++
	tracked[pid] = trackSeq
	return trackSeq
}

func isTracked(pid int) bool {
	trackedMu.Lock()
	defer trackedMu.Unlock()
	_, ok := tracked[pid]
	return ok
}
