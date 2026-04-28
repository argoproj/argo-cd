package commands

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/sandbox"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

func NewCommand() *cobra.Command {
	var (
		configPath         string
		allowRules         []string
		useImplementations []string
	)

	command := cobra.Command{
		Use:               common.CommandSandbox,
		Short:             "Argo Tool Execution Sandbox",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, cmdargs []string) {
			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			if len(cmdargs) < 1 {
				errors.CheckError(fmt.Errorf("expected at least 1 argument, got %d", len(cmdargs)-1))
			}

			log.Infof("argocd-sandbox started")
			log.Infof("  allow rules (%d) %v", len(allowRules), allowRules)
			sandboxCfg, err := sandbox.ReadSandboxConfig(configPath)
			errors.CheckError(err)
			log.Infof("executing %v", cmdargs)
			err = sandbox.ExecuteCommand(sandboxCfg, allowRules, cmdargs)
			// Normally the process is replaced, we won't get there
			log.Errorf("Failed to execute command: %v", err)
			os.Exit(2)
		},
	}
	command.Flags().SetInterspersed(false)
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_SANDBOX_LOGFORMAT", "json"), "Set the logging format. One of: json|text")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_SANDBOX_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&configPath, "config", "", "Set configuration file location")
	// not split by
	command.Flags().StringArrayVar(&allowRules, "allow", []string{}, "allow access to a resource")
	command.Flags().StringSliceVar(&useImplementations, "impl", []string{}, "Use sandbox implementations")
	command.MarkFlagRequired("config")
	return &command

}
