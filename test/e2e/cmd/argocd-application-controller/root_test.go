// +build e2efixtures

package main

import (
	"testing"

	"github.com/argoproj/argo-cd/cmd/argocd-application-controller/commands"

	"github.com/argoproj/argo-cd/test/e2e/cmd"
)

func TestArgoCDServer(t *testing.T) {
	cmd.Wrap(func() error {
		return commands.NewCommand().Execute()
	})
}
