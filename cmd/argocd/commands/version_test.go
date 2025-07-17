package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/version"
)

func TestShortVersionClient(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, nil)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version", "--short", "--client"})
	require.NoError(t, cmd.Execute(), "Failed to execute short version command")
	assert.Equal(t, "argocd: v99.99.99+unknown\n", buf.String())
}

func TestShortVersion(t *testing.T) {
	serverVersion := &version.VersionMessage{Version: "v99.99.99+unknown"}
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, serverVersion)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"argocd", "version", "--short"})
	require.NoError(t, cmd.Execute(), "Failed to execute short version command")
	assert.Equal(t, "argocd: v99.99.99+unknown\nargocd-server: v99.99.99+unknown\n", buf.String())
}
