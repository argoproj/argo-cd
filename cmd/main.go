package main

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"go.uber.org/automaxprocs/maxprocs"

	"github.com/spf13/cobra"

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
	setAutoMaxProcs(binaryName)

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

// setAutoMaxProcs sets the GOMAXPROCS value based on the binary name.
// It suppresses logs for CLI binaries and logs the setting for services.
func setAutoMaxProcs(binaryName string) {
	isCLI := binaryName == "argocd" ||
		binaryName == "argocd-linux-amd64" ||
		binaryName == "argocd-darwin-amd64" ||
		binaryName == "argocd-windows-amd64.exe"

	if isCLI {
		_, _ = maxprocs.Set() // Intentionally ignore errors for CLI binaries
	} else {
		_, err := maxprocs.Set(maxprocs.Logger(log.Infof))
		if err != nil {
			log.Errorf("Error setting GOMAXPROCS: %v", err)
		}
	}
}
