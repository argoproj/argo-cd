package commands

import (
	"bytes"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
)

func TestShortVersion(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := NewVersionCmd(&argocdclient.ClientOptions{
		ServerAddr: "localhost:80",
		PlainText:  true,
	})
	cmd.SetOutput(buf)
	cmd.SetArgs([]string{"version", "--short", "--client"})
	err := cmd.Execute()
	if err != nil {
		t.Fatal("Failed to execute short version command")
	}
	output := buf.String()
	if string(output) != "argocd: v99.99.99+unknown\n" {
		t.Fatalf("expected \"%s\" got \"%s\"", "argocd: v99.99.99+unknown\n", output)
	}
}
