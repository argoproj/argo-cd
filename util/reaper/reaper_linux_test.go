package reaper

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang.org/x/sys/unix"
)

// becomeSubreaper makes the test process adopt orphaned descendants the same
// way PID 1 does in a container, so the reaper can be exercised without
// actually running as PID 1.
func becomeSubreaper(t *testing.T) {
	t.Helper()
	err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)
	require.NoError(t, err, "PR_SET_CHILD_SUBREAPER must be available")
	t.Cleanup(func() {
		_ = unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 0, 0, 0, 0)
	})
}

// spawnOrphan forks a shell whose grandchild (`sleep 0.1`) outlives it and
// gets reparented to this process, then exits ~0.1s later. It returns the
// grandchild's pid. The shell itself is spawned tracked, the same way
// cmpserver/plugin spawns plugin commands, so a concurrently running reaper
// cannot steal its wait.
func spawnOrphan(t *testing.T) int {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sh", "-c", `( sleep 0.1 & echo $! )`)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	token, err := StartTracked(cmd)
	require.NoError(t, err)
	defer Untrack(cmd.Process.Pid, token)
	require.NoError(t, cmd.Wait())
	pid, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	require.NoError(t, err)
	return pid
}

func statOf(pid int) (state byte, ppid int, ok bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, 0, false
	}
	return parseProcStat(string(data))
}

// TestReapOrphans_ReapsOrphanedZombies demonstrates the bug and the fix in
// one test: without reaping, an orphaned child reparented to us stays a
// zombie forever (the PID-1 leak from the issue); reapOrphans collects it.
func TestReapOrphans_ReapsOrphanedZombies(t *testing.T) {
	becomeSubreaper(t)

	pid := spawnOrphan(t)

	// Defect demonstrated: with no reaper running, the orphan becomes a
	// zombie child of ours and nothing collects it. This is the state that
	// accumulates unboundedly in the CMP sidecar without the fix.
	require.Eventually(t, func() bool {
		state, ppid, ok := statOf(pid)
		return ok && state == 'Z' && ppid == os.Getpid()
	}, 5*time.Second, 10*time.Millisecond, "orphan pid %d never became a zombie child of the test process", pid)

	reaped := reapOrphans()
	assert.GreaterOrEqual(t, reaped, 1, "reapOrphans should have reaped the orphan")

	_, _, ok := statOf(pid)
	assert.False(t, ok, "zombie should be gone from /proc after reaping")
}

// TestReaper_DoesNotStealTrackedWaits runs the reap pass at full tilt while
// tracked commands (the way cmpserver/plugin spawns plugin commands) run
// concurrently, and asserts no cmd.Wait fails with a stolen-wait ECHILD.
func TestReaper_DoesNotStealTrackedWaits(t *testing.T) {
	becomeSubreaper(t)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				reapOrphans()
			}
		}
	}()

	var wg sync.WaitGroup
	errs := make(chan error, 200)
	for range 200 {
		wg.Go(func() {
			cmd := exec.CommandContext(t.Context(), "true")
			token, err := StartTracked(cmd)
			if err != nil {
				errs <- err
				return
			}
			defer Untrack(cmd.Process.Pid, token)
			if err := cmd.Wait(); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err, "tracked command wait must never be stolen by the reaper")
	}
}

// TestRun_ReapsOnSIGCHLD exercises the SIGCHLD-driven loop end to end.
func TestRun_ReapsOnSIGCHLD(t *testing.T) {
	becomeSubreaper(t)

	start(t.Context())

	pid := spawnOrphan(t)

	assert.Eventually(t, func() bool {
		_, _, ok := statOf(pid)
		return !ok
	}, 10*time.Second, 20*time.Millisecond, "reaper loop should collect the orphaned zombie")
}

func TestParseProcStat(t *testing.T) {
	tests := []struct {
		name      string
		stat      string
		wantState byte
		wantPpid  int
		wantOk    bool
	}{
		{"zombie child of pid 1", "42 (git) Z 1 42 1 0 -1 4227084 0 0 0 0 0 0 0 0 20 0 1 0 100 0 0", 'Z', 1, true},
		{"running process", "10 (kustomize) R 7 10 7 0 -1 4194304 100 0 0 0 1 1 0 0 20 0 1 0 50 1000 10", 'R', 7, true},
		{"comm with spaces and parens", "99 (weird (comm) name) S 3 99 3 0 -1 0 0 0 0 0 0 0 0 0 20 0 1 0 1 0 0", 'S', 3, true},
		{"truncated", "42 (git)", 0, 0, false},
		{"garbage", "not a stat line", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, ppid, ok := parseProcStat(tt.stat)
			assert.Equal(t, tt.wantOk, ok)
			if tt.wantOk {
				assert.Equal(t, tt.wantState, state)
				assert.Equal(t, tt.wantPpid, ppid)
			}
		})
	}
}
