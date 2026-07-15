package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

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
	"github.com/argoproj/argo-cd/v3/common"
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
	// Only the user-facing CLI gets signal-aware context wiring: server binaries
	// install their own handlers, and NotifyContext would swallow their SIGINT/SIGTERM.
	handleSignals := false

	switch binaryName {
	case common.CommandCLI:
		command = cli.NewCommand()
		isArgocdCLI = true
		handleSignals = true
	case common.CommandServer:
		command = apiserver.NewCommand()
	case common.CommandApplicationController:
		command = appcontroller.NewCommand()
	case common.CommandRepoServer:
		command = reposerver.NewCommand()
	case common.CommandCMPServer:
		command = cmpserver.NewCommand()
		isArgocdCLI = true
	case common.CommandCommitServer:
		command = commitserver.NewCommand()
	case common.CommandDex:
		command = dex.NewCommand()
	case common.CommandNotifications:
		command = notification.NewCommand()
	case common.CommandGitAskPass:
		command = gitaskpass.NewCommand()
		isArgocdCLI = true
	case common.CommandApplicationSetController:
		command = applicationset.NewCommand()
	case common.CommandK8sAuth:
		command = k8sauth.NewCommand()
		isArgocdCLI = true
	default:
		// "argocd-linux-amd64", "argocd-darwin-amd64", "argocd-windows-amd64.exe" are also valid binary names
		command = cli.NewCommand()
		isArgocdCLI = true
		handleSignals = true
	}

	if isArgocdCLI {
		// silence errors and usages since we'll be printing them manually.
		// This is because if we execute a plugin, the initial
		// errors and usage are always going to get printed that we don't want.
		command.SilenceErrors = true
		command.SilenceUsage = true
	}

	ctx := context.Background()
	if handleSignals {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		go func() {
			// second Ctrl-C restores default signal behavior and force-terminates
			<-ctx.Done()
			stop()
		}()
	}

	err := command.ExecuteContext(ctx)
	// if an error is present, try to look for various scenarios
	// such as if the error is from the execution of a normal argocd command,
	// unknown command error or any other.
	if err != nil {
		errMsg, pluginErr := cli.NewDefaultPluginHandler().HandleCommandExecutionError(err, isArgocdCLI, os.Args)
		if pluginErr != nil {
			os.Stdout.WriteString(errMsg)
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
