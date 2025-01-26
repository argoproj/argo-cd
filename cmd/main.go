package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	appcontroller "github.com/argoproj/argo-cd/v3/cmd/argocd-application-controller/commands"
	applicationset "github.com/argoproj/argo-cd/v3/cmd/argocd-applicationset-controller/commands"
	cmpserver "github.com/argoproj/argo-cd/v3/cmd/argocd-cmp-server/commands"
	commitserver "github.com/argoproj/argo-cd/v3/cmd/argocd-commit-server/commands"
	dex "github.com/argoproj/argo-cd/v3/cmd/argocd-dex/commands"
	gitaskpass "github.com/argoproj/argo-cd/v3/cmd/argocd-git-ask-pass/commands"
	k8sauth "github.com/argoproj/argo-cd/v3/cmd/argocd-k8s-auth/commands"
	notification "github.com/argoproj/argo-cd/v3/cmd/argocd-notification/commands"
	reposerver "github.com/argoproj/argo-cd/v3/cmd/argocd-repo-server/commands"
	apiserver "github.com/argoproj/argo-cd/v3/cmd/argocd-server/commands"
	cli "github.com/argoproj/argo-cd/v3/cmd/argocd/commands"
	"github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/util/log"
)

const (
	binaryNameEnv = "ARGOCD_BINARY_NAME"
)

func init() {
	// Make sure klog uses the configured log level and format.
	klog.SetLogger(log.NewLogrusLogger(log.NewWithCurrentConfig()))
}

func main() {
	var command *cobra.Command

	binaryName := filepath.Base(os.Args[0])
	if val := os.Getenv(binaryNameEnv); val != "" {
		binaryName = val
	}

	isCLI := false
	switch binaryName {
	case "argocd", "argocd-linux-amd64", "argocd-darwin-amd64", "argocd-windows-amd64.exe":
		command = cli.NewCommand()
		isCLI = true
	case "argocd-server":
		command = apiserver.NewCommand()
	case "argocd-application-controller":
		command = appcontroller.NewCommand()
	case "argocd-repo-server":
		command = reposerver.NewCommand()
	case "argocd-cmp-server":
		command = cmpserver.NewCommand()
		isCLI = true
	case "argocd-commit-server":
		command = commitserver.NewCommand()
	case "argocd-dex":
		command = dex.NewCommand()
	case "argocd-notifications":
		command = notification.NewCommand()
	case "argocd-git-ask-pass":
		command = gitaskpass.NewCommand()
		isCLI = true
	case "argocd-applicationset-controller":
		command = applicationset.NewCommand()
	case "argocd-k8s-auth":
		command = k8sauth.NewCommand()
		isCLI = true
	default:
		command = cli.NewCommand()
		isCLI = true
	}
	util.SetAutoMaxProcs(isCLI)

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
