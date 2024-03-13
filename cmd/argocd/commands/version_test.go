package commands

import (
	"bytes"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/version"
	"github.com/stretchr/testify/assert"
)

func TestShortVersionClient(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, nil)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version", "--short", "--client"})
	err := cmd.Execute()
	if err != nil {
		t.Fatal("Failed to execute short version command")
	}
	output := buf.String()
	assert.Equal(t, output, "argocd: v99.99.99+unknown\n")
}

func TestShortVersion(t *testing.T) {
	serverVersion := &version.VersionMessage{Version: "v99.99.99+unknown"}
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{}, serverVersion)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"argocd", "version", "--short"})
	err := cmd.Execute()
	if err != nil {
		t.Fatal("Failed to execute short version command")
	}
	output := buf.String()
	assert.Equal(t, output, "argocd: v99.99.99+unknown\nargocd-server: v99.99.99+unknown\n")
}
