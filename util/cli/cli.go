// Package cmd provides functionally common to various argo CLIs

package cli

import (
	"fmt"
	"os"

	argocd "github.com/argoproj/argo-cd"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// NewVersionCmd returns a new `version` command to be used as a sub-command to root
func NewVersionCmd(cliName string) *cobra.Command {
	var short bool
	versionCmd := cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Print version information"),
		Run: func(cmd *cobra.Command, args []string) {
			version := argocd.GetVersion()
			fmt.Printf("%s: %s\n", cliName, version)
			if short {
				return
			}
			fmt.Printf("  BuildDate: %s\n", version.BuildDate)
			fmt.Printf("  GitCommit: %s\n", version.GitCommit)
			fmt.Printf("  GitTreeState: %s\n", version.GitTreeState)
			if version.GitTag != "" {
				fmt.Printf("  GitTag: %s\n", version.GitTag)
			}
			fmt.Printf("  GoVersion: %s\n", version.GoVersion)
			fmt.Printf("  Compiler: %s\n", version.Compiler)
			fmt.Printf("  Platform: %s\n", version.Platform)
		},
	}
	versionCmd.Flags().BoolVar(&short, "short", false, "print just the version number")
	return &versionCmd
}

// AddKubectlFlagsToCmd adds kubectl like flags to a command and returns the ClientConfig interface
// for retrieving the values.
func AddKubectlFlagsToCmd(cmd *cobra.Command) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, cmd.PersistentFlags(), kflags)
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}
