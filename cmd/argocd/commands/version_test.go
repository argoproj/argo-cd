package commands

import (
	"bytes"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/stretchr/testify/assert"
)

func TestShortVersion(t *testing.T) {
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
