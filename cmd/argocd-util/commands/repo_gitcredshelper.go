package commands

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/git"
)

func NewGitCredsHelperCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		glogLevel    int
	)
	var command = &cobra.Command{
		Use:               "gitcredshelper",
		Short:             "Identify credentials for Git repositories for on-host Git.",
		Long:              "The ArgoCD Git Credential Helper is a Git credential helper function backed by the ArgoCD Repo Server.  It implements the Git credential helper interface (https://git-scm.com/docs/gitcredentials#_custom_helpers) and extends the provision of repo credentials to on-host Git operations.

It is not intended for direct use on the command line.",
		DisableAutoGenTag: true,
		Args: cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			if args[0] != "get" {
				return
			}

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeclientset := kubernetes.NewForConfigOrDie(config)

			argoCDOpts := git.ArgoCDGitCredentialHelperOpts{
				Namespace:           namespace,
				KubeClientset:       kubeclientset,
			}

			for {
				ctx := context.Background()
				ctx, cancel := context.WithCancel(ctx)
				credsHelper := git.NewGitCredentialHelper(ctx, argoCDOpts)
				credsHelper.Run(ctx)
				cancel()
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_REPO_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	return command
}
