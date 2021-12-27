package main

import (
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/hack/gen-resources/cmd/commands"
)

func main() {
	command := commands.NewCommand()
	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
