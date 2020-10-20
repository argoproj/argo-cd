package main

import (
	"log"
	"os"

	"github.com/argoproj/argo-cd/cmd/argocd/commands"

	"github.com/spf13/cobra/doc"
)

func main() {
	// set HOME env var so that default values involve user's home directory do not depend on the running user.
	os.Setenv("HOME", "/home/user")

	err := doc.GenMarkdownTree(commands.NewCommand(), "./docs/user-guide/commands")
	if err != nil {
		log.Fatal(err)
	}
}
