package main

import (
	"errors"
	"os"
	"os/exec"
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

	isArgocdCLI := false

	switch binaryName {
	case "argocd", "argocd-linux-amd64", "argocd-darwin-amd64", "argocd-windows-amd64.exe":
		command = cli.NewCommand()
		isArgocdCLI = true
	case "argocd-server":
		command = apiserver.NewCommand()
	case "argocd-application-controller":
		command = appcontroller.NewCommand()
	case "argocd-repo-server":
		command = reposerver.NewCommand()
	case "argocd-cmp-server":
		command = cmpserver.NewCommand()
		isArgocdCLI = true
	case "argocd-commit-server":
		command = commitserver.NewCommand()
	case "argocd-dex":
		command = dex.NewCommand()
	case "argocd-notifications":
		command = notification.NewCommand()
	case "argocd-git-ask-pass":
		command = gitaskpass.NewCommand()
		isArgocdCLI = true
	case "argocd-applicationset-controller":
		command = applicationset.NewCommand()
	case "argocd-k8s-auth":
		command = k8sauth.NewCommand()
		isArgocdCLI = true
	default:
		command = cli.NewCommand()
		isArgocdCLI = true
	}
	util.SetAutoMaxProcs(isArgocdCLI)

	if isArgocdCLI {
		// silence errors and usages since we'll be printing them manually.
		// This is because if we execute a plugin, the initial
		// errors and usage are always going to get printed that we don't want.
		command.SilenceErrors = true
		command.SilenceUsage = true
	}

	err := command.Execute()
	// if the err is non-nil, try to look for various scenarios
	// such as if the error is from the execution of a normal argocd command,
	// unknown command error or any other.
	if err != nil {
		pluginHandler := cli.NewDefaultPluginHandler([]string{"argocd"})
		pluginErr := pluginHandler.HandleCommandExecutionError(err, isArgocdCLI, os.Args)
		if pluginErr != nil {
			var exitErr *exec.ExitError
			if errors.As(pluginErr, &exitErr) {
				// Return the actual plugin exit code
				os.Exit(exitErr.ExitCode())
			}
			// Fallback to exit code 1 if the error isn't an exec.ExitError
			os.Exit(1)
		}
	}
}
