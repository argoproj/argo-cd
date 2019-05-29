package main

import (
	"fmt"
	"github.com/argoproj/argo-cd/cmd/argocd-repo-server/commands"
	"os"
)


func main() {
	if err := commands.NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
