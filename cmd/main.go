package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	_ "go.uber.org/automaxprocs"

	appcontroller "github.com/argoproj/argo-cd/v2/cmd/argocd-application-controller/commands"
	applicationset "github.com/argoproj/argo-cd/v2/cmd/argocd-applicationset-controller/commands"
	cmpserver "github.com/argoproj/argo-cd/v2/cmd/argocd-cmp-server/commands"
	dex "github.com/argoproj/argo-cd/v2/cmd/argocd-dex/commands"
	gitaskpass "github.com/argoproj/argo-cd/v2/cmd/argocd-git-ask-pass/commands"
	k8sauth "github.com/argoproj/argo-cd/v2/cmd/argocd-k8s-auth/commands"
	notification "github.com/argoproj/argo-cd/v2/cmd/argocd-notification/commands"
	reposerver "github.com/argoproj/argo-cd/v2/cmd/argocd-repo-server/commands"
	apiserver "github.com/argoproj/argo-cd/v2/cmd/argocd-server/commands"
	cli "github.com/argoproj/argo-cd/v2/cmd/argocd/commands"
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
	case "argocd-server":
		command = apiserver.NewCommand()
	case "argocd-application-controller":
		command = appcontroller.NewCommand()
	case "argocd-repo-server":
		command = reposerver.NewCommand()
	case "argocd-cmp-server":
		command = cmpserver.NewCommand()
	case "argocd-dex":
		command = dex.NewCommand()
	case "argocd-notifications":
		command = notification.NewCommand()
	case "argocd-git-ask-pass":
		command = gitaskpass.NewCommand()
	case "argocd-applicationset-controller":
		command = applicationset.NewCommand()
	case "argocd-k8s-auth":
		command = k8sauth.NewCommand()
	default:
		command = cli.NewCommand()
	}

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
