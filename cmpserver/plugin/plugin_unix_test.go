//go:build !windows

package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A descendant that redirects its output does not keep cmd.Wait blocked, so only
// the process group signal can stop it after the exec timeout.
func TestRunCommand_execTimeoutKillsDescendants(t *testing.T) {
	old := pluginExecTimeout
	pluginExecTimeout = 300 * time.Millisecond
	t.Cleanup(func() { pluginExecTimeout = old })

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	_, err := runCommand(t.Context(), Command{
		Command: []string{"sh", "-c", `sleep 30 >/dev/null 2>&1 & echo $! > "$0"; wait`, pidFile},
	}, "", nil)
	require.Error(t, err)

	data, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	assert.Eventually(t, func() bool {
		return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
	}, 4*time.Second, 50*time.Millisecond, "descendant process survived the exec timeout")
}
