package main

import (
	"log"

	"github.com/argoproj/argo-cd/cmd/argocd/commands"

	"github.com/spf13/cobra/doc"
)

func main() {
	err := doc.GenMarkdownTree(commands.NewCommand(), "./docs/user-guide/commands")
	if err != nil {
		log.Fatal(err)
	}
}
