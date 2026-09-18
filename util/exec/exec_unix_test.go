//go:build !windows

package exec

import (
	"context"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetChildProcessGroupIsolationDisabled covers the argocd CLI: a command moved out of the
// terminal's foreground process group stops receiving Ctrl-C.
func TestSetChildProcessGroupIsolationDisabled(t *testing.T) {
	t.Cleanup(func() { isolateProcessGroups = true })
	DisableProcessGroupIsolation()

	cmd := exec.CommandContext(t.Context(), "true")
	SetChildProcessGroup(cmd)
	assert.Nil(t, cmd.SysProcAttr)

	isolateProcessGroups = true
	SetChildProcessGroup(cmd)
	require.NotNil(t, cmd.SysProcAttr)
	assert.True(t, cmd.SysProcAttr.Setpgid)
}

// TestSignalProcessGroupSkipsGroupWithoutSetpgid covers the CLI, where the command shares the caller's
// process group: -PID would then name an unrelated group, and a recycled PID is all it takes for that
// group to exist and be signalled.
func TestSignalProcessGroupSkipsGroupWithoutSetpgid(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "sleep 30")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	var killed []int
	record := func(pid int, _ syscall.Signal) error {
		killed = append(killed, pid)
		return nil
	}

	// No SetChildProcessGroup: nothing may be addressed by negative PID.
	require.NoError(t, signalProcessGroup(cmd, syscall.SIGTERM, record))
	assert.Empty(t, killed, "signalled a process group the command does not lead")

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	require.NoError(t, signalProcessGroup(cmd, syscall.SIGTERM, record))
	assert.Equal(t, []int{-cmd.Process.Pid}, killed)
}

// TestTerminateGroupOnCancelReapsOrphanIgnoringSIGTERM covers the grandchild the cmpserver's fixed 5s
// SIGKILL used to reap: it ignores SIGTERM and redirects its output, so it holds none of the command's
// pipes and cmd.Wait returns as soon as the command itself dies - milliseconds after the cancellation,
// long before the escalation is due.
func TestTerminateGroupOnCancelReapsOrphanIgnoringSIGTERM(t *testing.T) {
	pidFile := path.Join(t.TempDir(), "orphan.pid")
	// The orphan reports itself only once its trap is installed, so the cancellation below cannot
	// race it and kill it with the plain SIGTERM. Redirected, so it holds none of the command's pipes
	// - those are what would otherwise keep Wait blocked until WaitDelay.
	script := `sh -c 'trap "" TERM; echo $$ > ` + pidFile + `; while :; do sleep 0.05; done' >/dev/null 2>&1 &
while :; do sleep 0.05; done`

	ctx, cancel := context.WithCancel(t.Context())
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	var sink strings.Builder
	cmd.Stdout, cmd.Stderr = &sink, &sink

	// Escalation lands at half of this. Generous, so that a slow Wait on a loaded runner cannot make
	// the "still alive" check below race the SIGKILL it is the precondition for.
	grace := 5 * time.Second
	stop := TerminateGroupOnCancel(cmd, grace)
	require.NoError(t, cmd.Start())

	var orphan int
	require.Eventually(t, func() bool {
		raw, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		orphan, err = strconv.Atoi(strings.TrimSpace(string(raw)))
		return err == nil && orphan > 0
	}, 10*time.Second, 5*time.Millisecond, "orphan never reported itself")
	t.Cleanup(func() { _ = syscall.Kill(orphan, syscall.SIGKILL) })

	cancel()
	_ = cmd.Wait()
	stop()

	// Without this the test would pass on an orphan that simply died with the group, never exercising
	// the escalation.
	require.NoError(t, syscall.Kill(orphan, 0), "orphan did not survive the SIGTERM, so nothing was escalated against")

	assert.Eventually(t, func() bool {
		return syscall.Kill(orphan, 0) != nil
	}, 5*grace, 20*time.Millisecond, "orphan survived the escalation")
}

// TestRunCommandExtCancelSignalsGroup covers the commands that install no policy of their own - helm
// and kustomize - whose grandchildren os/exec would otherwise orphan: `kustomize build` runs git for
// remote bases, and that git is what leaves .git/index.lock behind when nothing signals it.
func TestRunCommandExtCancelSignalsGroup(t *testing.T) {
	dir := t.TempDir()
	ready, marker := path.Join(dir, "ready"), path.Join(dir, "signalled")
	// The grandchild redirects its output, so it is not one of the processes cmd.Wait blocks on, and
	// reports the SIGTERM rather than dying of it. It reports readiness only once its trap is
	// installed, so the cancellation below cannot race it.
	// Short sleeps rather than one long one: a shell defers a trap until the running command finishes,
	// so a SIGTERM landing just before `sleep 30` starts would not be serviced for 30s - the command
	// was started after the signal, so it never receives one of its own.
	script := `sh -c 'trap "touch ` + marker + `; exit" TERM; touch ` + ready + `; while :; do sleep 0.05; done' >/dev/null 2>&1 &
while :; do sleep 0.05; done`

	ctx, cancel := context.WithCancel(t.Context())
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	// No TerminateGroupOnCancel here: RunCommandExt has to install it.
	errCh := make(chan error, 1)
	go func() {
		_, err := RunCommandExt(cmd, CmdOpts{Timeout: time.Minute})
		errCh <- err
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	}, 10*time.Second, 5*time.Millisecond, "grandchild never started")
	cancel()

	select {
	case <-errCh:
	case <-time.After(30 * time.Second):
		t.Fatal("RunCommandExt never returned")
	}

	assert.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "grandchild was never signalled: the group was orphaned")
}
