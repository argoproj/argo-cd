//go:build !race

package exec

import (
	"os/exec"
	"runtime"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test lives here, rather than beside the rest, for the same reason
// TestRunCommandTimeout does: it takes the timeout path that reads the output
// buffers while os/exec is still writing to them. A later commit fixes that
// race and moves both tests back.
func TestRunCommandExt_timeoutWithoutWaitDoesNotLeakGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()
	for range 5 {
		cmd := exec.CommandContext(t.Context(), "sleep", "5")
		_, err := RunCommandExt(cmd, CmdOpts{Timeout: 100 * time.Millisecond})
		require.Error(t, err)
	}
	// Eventually calls the condition from its own goroutine, so the count is one higher while it runs.
	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= before+1
	}, 2*time.Second, 20*time.Millisecond)
}

// FIXME: there's a race in RunCommand that causes the test to fail when -race is enabled. The race is in the timeout
// handling, which shares bytes buffers between the exec goroutine and the timeout handler code.
func TestRunCommandTimeout(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	output, err := RunCommand("sh", CmdOpts{Timeout: 500 * time.Millisecond}, "-c", "echo hello world && echo my-error >&2 && sleep 2")
	assert.Equal(t, "hello world", output)
	require.EqualError(t, err, "`sh -c echo hello world && echo my-error >&2 && sleep 2` failed timeout after 500ms")

	assert.Len(t, hook.Entries, 3)

	entry := hook.Entries[0]
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, "sh -c echo hello world && echo my-error >&2 && sleep 2", entry.Message)
	assert.Contains(t, entry.Data, "dir")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[1]
	assert.Equal(t, log.DebugLevel, entry.Level)
	assert.Equal(t, "hello world\n", entry.Message)
	assert.Contains(t, entry.Data, "duration")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[2]
	assert.Equal(t, log.ErrorLevel, entry.Level)
	assert.Equal(t, "`sh -c echo hello world && echo my-error >&2 && sleep 2` failed timeout after 500ms", entry.Message)
	assert.Contains(t, entry.Data, "execID")
}
