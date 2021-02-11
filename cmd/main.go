package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	appcontroller "github.com/argoproj/argo-cd/cmd/argocd-application-controller/commands"
	dex "github.com/argoproj/argo-cd/cmd/argocd-dex/commands"
	reposerver "github.com/argoproj/argo-cd/cmd/argocd-repo-server/commands"
	apiserver "github.com/argoproj/argo-cd/cmd/argocd-server/commands"
	util "github.com/argoproj/argo-cd/cmd/argocd-util/commands"
	cli "github.com/argoproj/argo-cd/cmd/argocd/commands"
)

const (
	binaryNameEnv = "ARGOCD_BINARY_NAME"
)

func main() {
	var command *cobra.Command

	binaryName := filepath.Base(os.Args[0])
	if val := os.Getenv(binaryNameEnv); val != "" {
		binaryName = val
	}
	switch binaryName {
	case "argocd", "argocd-linux-amd64", "argocd-darwin-amd64", "argocd-windows-amd64.exe":
		command = cli.NewCommand()
	case "argocd-util", "argocd-util-linux-amd64", "argocd-util-darwin-amd64", "argocd-util-windows-amd64.exe":
		command = util.NewCommand()
	case "argocd-server":
		command = apiserver.NewCommand()
	case "argocd-application-controller":
		command = appcontroller.NewCommand()
	case "argocd-repo-server":
		command = reposerver.NewCommand()
	case "argocd-dex":
		command = dex.NewCommand()
	default:
		if len(os.Args[1:]) > 0 {
			// trying to guess between argocd and argocd-util by matching sub command
			for _, cmd := range []*cobra.Command{cli.NewCommand(), util.NewCommand()} {
				if _, _, err := cmd.Find(os.Args[1:]); err == nil {
					command = cmd
					break
				}
			}
		}

		if command == nil {
			fmt.Printf("Unknown binary name '%s'.Use '%s' environment variable to specify required binary name "+
				"(possible values 'argocd' or 'argocd-util').\n", binaryName, binaryNameEnv)
			os.Exit(1)
		}
	}

	if err := command.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
