package main

import (
	commands "github.com/argoproj/argo-cd/cmd/argocd/commands"
	"github.com/argoproj/argo-cd/errors"
)

func main() {
	err := commands.NewCommand().Execute()
	errors.CheckError(err)
}
