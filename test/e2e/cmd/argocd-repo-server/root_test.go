// +build e2efixtures

package main

import (
	"testing"

	"github.com/argoproj/argo-cd/cmd/argocd-repo-server/commands"
	"github.com/argoproj/argo-cd/test/e2e/cmd"
)

func TestArgoCDRepoServer(t *testing.T) {
	cmd.Wrap(func() error {
		return commands.NewCommand().Execute()
	})
}
