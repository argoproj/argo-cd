package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/engine/pkg/utils/errors"
	argoio "github.com/argoproj/argo-cd/engine/pkg/utils/io"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apiclient/version"
)

// NewVersionCmd returns a new `version` command to be used as a sub-command to root
func NewVersionCmd(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		short  bool
		client bool
		output string
	)

	versionCmd := cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Print version information"),
		Example: `  # Print the full version of client and server to stdout
  argocd version

  # Print only full version of the client - no connection to server will be made
  argocd version --client

  # Print the full version of client and server in JSON format
  argocd version -o json

  # Print only client and server core version strings in YAML format
  argocd version --short -o yaml
`,
		Run: func(cmd *cobra.Command, args []string) {
			var (
				versionIf  version.VersionServiceClient
				serverVers *version.VersionMessage
				conn       io.Closer
				err        error
			)
			if !client {
				// Get Server version
				conn, versionIf = argocdclient.NewClientOrDie(clientOpts).NewVersionClientOrDie()
				defer argoio.Close(conn)
				serverVers, err = versionIf.Version(context.Background(), &empty.Empty{})
				errors.CheckError(err)
			}
			switch output {
			case "yaml", "json":
				clientVers := common.GetVersion()
				version := make(map[string]interface{})
				if !short {
					version["client"] = clientVers
				} else {
					version["client"] = map[string]string{cliName: clientVers.Version}
				}
				if !client {
					if !short {
						version["server"] = serverVers
					} else {
						version["server"] = map[string]string{"argocd-server": serverVers.Version}
					}
				}
				err := PrintResource(version, output)
				errors.CheckError(err)
			case "short":
				printVersion(serverVers, client, true)
			case "wide", "":
				// we use value of short for backward compatibility
				printVersion(serverVers, client, short)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	versionCmd.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|short")
	versionCmd.Flags().BoolVar(&short, "short", false, "print just the version number")
	versionCmd.Flags().BoolVar(&client, "client", false, "client version only (no server required)")
	return &versionCmd
}

func printVersion(serverVers *version.VersionMessage, client bool, short bool) {
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
		fmt.Printf("  Kustomize Version: %s\n", serverVers.KustomizeVersion)
		fmt.Printf("  Helm Version: %s\n", serverVers.HelmVersion)
		fmt.Printf("  Kubectl Version: %s\n", serverVers.KubectlVersion)
	}
}
