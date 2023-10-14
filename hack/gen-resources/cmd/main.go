package main

import (
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/hack/gen-resources/cmd/commands"
	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	command := commands.NewCommand()
	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
