package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCommand_OTLPHeadersFlagUsesEnv proves the otlp-headers flag default is wired to the
// canonical ARGOCD_REPO_SERVER_OTLP_HEADERS environment variable.
func TestNewCommand_OTLPHeadersFlagUsesEnv(t *testing.T) {
	t.Setenv("ARGOCD_REPO_SERVER_OTLP_HEADERS", "traceparent=abc123")

	cmd := NewCommand()

	got, err := cmd.Flags().GetStringToString("otlp-headers")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"traceparent": "abc123"}, got)
}