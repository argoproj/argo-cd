package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenRepoSpecCommand_SparsePaths(t *testing.T) {
	cmd := NewGenRepoSpecCommand()
	cmd.SetArgs([]string{"https://github.com/argoproj/argo-cd", "--sparse-paths", "charts", "--sparse-paths", "apps"})

	out, err := captureStdout(func() {
		require.NoError(t, cmd.Execute())
	})
	require.NoError(t, err)
	assert.Contains(t, out, "sparsePaths: charts,apps")
}
