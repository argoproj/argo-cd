package main

import (
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/cmd/argocd-repo-server/commands"
)

func main() {
	if err := commands.NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
