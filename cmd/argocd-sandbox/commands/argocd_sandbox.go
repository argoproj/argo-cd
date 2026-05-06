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
		configStr          string
		allowRules         []string
		useImplementations []string
		landlockAllowArgs  []string
	)
	const (
		FLAG_CONFIG     = "config"
		FLAG_CONFIG_STR = "config-str"
	)

	command := cobra.Command{
		Use:               common.CommandSandbox,
		Short:             "Argo Tool Execution Sandbox",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, cmdargs []string) {
			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			if len(cmdargs) < 1 {
				errors.Fatal(errors.ErrorGeneric,
					fmt.Errorf("expected at least 1 argument, got %d", len(cmdargs)))
			}

			log.Infof("argocd-sandbox started")
			log.Infof("  allow rules (%d) %v", len(allowRules), allowRules)
			var err error
			var sandboxCfg *sandbox.ArgocdSandboxConfig
			if configPath != "" {
				sandboxCfg, err = sandbox.ReadSandboxConfig(configPath)
			}
			if configStr != "" {
				sandboxCfg, err = sandbox.ReadSandboxConfigStr(configStr)
			}
			errors.CheckError(err)
			allowArgs := make(map[string][]string)
			allowArgs[sandbox.LANDLOCK] = landlockAllowArgs
			log.Infof("executing %v", cmdargs)
			err = sandbox.ExecuteCommand(sandboxCfg, useImplementations, allowArgs, cmdargs)
			// Normally the process is replaced, we won't get there
			log.Errorf("Failed to execute command: %v", err)
			os.Exit(2)
		},
	}
	command.Flags().SetInterspersed(false)
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_SANDBOX_LOGFORMAT", "json"), "Set the logging format. One of: json|text")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_SANDBOX_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&configPath, FLAG_CONFIG, "", "Set sandbox configuration file location")
	command.Flags().StringVar(&configStr, FLAG_CONFIG_STR, "", "Set sandbox configuration from command argument")
	// not split by
	command.Flags().StringArrayVar(&landlockAllowArgs, "landlock-allow", []string{}, "allow access to a resource using the Landlock module")
	command.Flags().StringSliceVar(&useImplementations, "impl", []string{}, "Use sandbox implementations")
	command.MarkFlagsOneRequired(FLAG_CONFIG, FLAG_CONFIG_STR)
	command.MarkFlagsMutuallyExclusive(FLAG_CONFIG, FLAG_CONFIG_STR)
	return &command

}
