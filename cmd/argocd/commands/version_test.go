package commands

import (
	"bytes"
	"context"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/version"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestShortVersionClient(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{})
	cmd.SetOutput(buf)
	cmd.SetArgs([]string{"version", "--short", "--client"})
	err := cmd.Execute()
	if err != nil {
		t.Fatal("Failed to execute short version command")
	}
	output := buf.String()
	assert.Equal(t, output, "argocd: v99.99.99+unknown\n")
}

func TestShortVersion(t *testing.T) {
	serverVersion := "argocd: v99.99.99+unknown"
	oldVersionFun := getServerVersion
	defer func() { getServerVersion = oldVersionFun }()
	getServerVersion = func(ctx context.Context, options *argocdclient.ClientOptions, c *cobra.Command) *version.VersionMessage {
		return &version.VersionMessage{Version: serverVersion}
	}
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{})
	cmd.SetOutput(buf)
	cmd.SetArgs([]string{"argocd", "version", "--short"})
	err := cmd.Execute()
	if err != nil {
		t.Fatal("Failed to execute short version command")
	}
	output := buf.String()
	assert.Equal(t, output, "argocd: v99.99.99+unknown\nargocd-server: argocd: v99.99.99+unknown\n")
}
