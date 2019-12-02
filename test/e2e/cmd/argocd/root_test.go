// +build e2efixtures

package main

import (
	"testing"

	"github.com/argoproj/argo-cd/cmd/argocd/commands"
	"github.com/argoproj/argo-cd/test/e2e/cmd"
)

func TestArgoCD(t *testing.T) {
	cmd.RunCommand(func() error {
		return commands.NewCommand().Execute()
	})
}
