//go:build !windows

package exec

import (
	"os/exec"
	"testing"

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
