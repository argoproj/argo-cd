package main

import (
	commands "github.com/argoproj/argo-cd/cmd/argocd-server/commands"
	"github.com/argoproj/argo-cd/errors"
)

func main() {
	err := commands.NewCommand().Execute()
	errors.CheckError(err)
}
