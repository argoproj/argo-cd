package commands

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util"
)

// NewVersionCmd returns a new `version` command to be used as a sub-command to root
func NewVersionCmd(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var short bool
	var client bool

	versionCmd := cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Print version information"),
		Run: func(cmd *cobra.Command, args []string) {
			version := common.GetVersion()
			fmt.Printf("%s: %s\n", cliName, version)
			if !short {
				fmt.Printf("  BuildDate: %s\n", version.BuildDate)
				fmt.Printf("  GitCommit: %s\n", version.GitCommit)
				fmt.Printf("  GitTreeState: %s\n", version.GitTreeState)
				if version.GitTag != "" {
					fmt.Printf("  GitTag: %s\n", version.GitTag)
				}
				fmt.Printf("  GoVersion: %s\n", version.GoVersion)
				fmt.Printf("  Compiler: %s\n", version.Compiler)
				fmt.Printf("  Platform: %s\n", version.Platform)
			}
			if client {
				return
			}

			// Get Server version
			conn, versionIf := argocdclient.NewClientOrDie(clientOpts).NewVersionClientOrDie()
			defer util.Close(conn)
			serverVers, err := versionIf.Version(context.Background(), &empty.Empty{})
			errors.CheckError(err)
			fmt.Printf("%s: %s\n", "argocd-server", serverVers.Version)
			if !short {
				fmt.Printf("  BuildDate: %s\n", serverVers.BuildDate)
				fmt.Printf("  GitCommit: %s\n", serverVers.GitCommit)
				fmt.Printf("  GitTreeState: %s\n", serverVers.GitTreeState)
				if version.GitTag != "" {
					fmt.Printf("  GitTag: %s\n", serverVers.GitTag)
				}
				fmt.Printf("  GoVersion: %s\n", serverVers.GoVersion)
				fmt.Printf("  Compiler: %s\n", serverVers.Compiler)
				fmt.Printf("  Platform: %s\n", serverVers.Platform)
				fmt.Printf("  Ksonnet Version: %s\n", serverVers.KsonnetVersion)
			}

		},
	}
	versionCmd.Flags().BoolVar(&short, "short", false, "print just the version number")
	versionCmd.Flags().BoolVar(&client, "client", false, "client version only (no server required)")
	return &versionCmd
}
