package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCommand_DisableTLSFlag(t *testing.T) {
	cmd := NewCommand()

	flag := cmd.Flags().Lookup("disable-tls")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)

	require.NoError(t, cmd.Flags().Set("disable-tls", "true"))
	value, err := cmd.Flags().GetBool("disable-tls")
	require.NoError(t, err)
	assert.True(t, value)
}

func TestNewCommand_DisableTLSAndClientCAPathAreMutuallyExclusive(t *testing.T) {
	t.Setenv("ARGOCD_EXEC_TIMEOUT", "1ms")

	cmd := NewCommand()
	cmd.SetArgs([]string{"--disable-tls", "--client-ca-path", "/tmp/client-ca.crt"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--client-ca-path cannot be used when --disable-tls is enabled")
}
