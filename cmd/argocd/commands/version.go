package commands

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
)

// NewVersionCmd returns a new `version` command to be used as a sub-command to root
func NewVersionCmd(clientOpts *argocdclient.ClientOptions, serverVersion *version.VersionMessage) *cobra.Command {
	var (
		short  bool
		client bool
		output string
	)

	versionCmd := cobra.Command{
		Use:   "version",
		Short: "Print version information",
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
			ctx := cmd.Context()

			cv := common.GetVersion()
			switch output {
			case "yaml", "json":
				v := make(map[string]interface{})

				if short {
					v["client"] = map[string]string{cliName: cv.Version}
				} else {
					v["client"] = cv
				}

				if !client {
					var sv *version.VersionMessage
					if serverVersion == nil {
						sv = getServerVersion(ctx, clientOpts, cmd)
					} else {
						sv = serverVersion
					}

					if short {
						v["server"] = map[string]string{"argocd-server": sv.Version}
					} else {
						v["server"] = sv
					}
				}

				err := PrintResource(v, output)
				errors.CheckError(err)
			case "wide", "short", "":
				fmt.Fprint(cmd.OutOrStdout(), printClientVersion(&cv, short || (output == "short")))
				if !client {
					var sv *version.VersionMessage
					if serverVersion == nil {
						sv = getServerVersion(ctx, clientOpts, cmd)
					} else {
						sv = serverVersion
					}
					fmt.Fprint(cmd.OutOrStdout(), printServerVersion(sv, short || (output == "short")))
				}
			default:
				log.Fatalf("unknown output format: %s", output)
			}
		},
	}
	versionCmd.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|short")
	versionCmd.Flags().BoolVar(&short, "short", false, "print just the version number")
	versionCmd.Flags().BoolVar(&client, "client", false, "client version only (no server required)")
	return &versionCmd
}

func getServerVersion(ctx context.Context, options *argocdclient.ClientOptions, c *cobra.Command) *version.VersionMessage {
	conn, versionIf := headless.NewClientOrDie(options, c).NewVersionClientOrDie()
	defer argoio.Close(conn)

	v, err := versionIf.Version(ctx, &empty.Empty{})
	errors.CheckError(err)

	return v
}

func printClientVersion(version *common.Version, short bool) string {
	output := fmt.Sprintf("%s: %s\n", cliName, version)
	if short {
		return output
	}
	output += fmt.Sprintf("  BuildDate: %s\n", version.BuildDate)
	output += fmt.Sprintf("  GitCommit: %s\n", version.GitCommit)
	output += fmt.Sprintf("  GitTreeState: %s\n", version.GitTreeState)
	if version.GitTag != "" {
		output += fmt.Sprintf("  GitTag: %s\n", version.GitTag)
	}
	output += fmt.Sprintf("  GoVersion: %s\n", version.GoVersion)
	output += fmt.Sprintf("  Compiler: %s\n", version.Compiler)
	output += fmt.Sprintf("  Platform: %s\n", version.Platform)
	if version.ExtraBuildInfo != "" {
		output += fmt.Sprintf("  ExtraBuildInfo: %s\n", version.ExtraBuildInfo)
	}
	return output
}

func printServerVersion(version *version.VersionMessage, short bool) string {
	output := fmt.Sprintf("%s: %s\n", "argocd-server", version.Version)

	if short {
		return output
	}

	if version.BuildDate != "" {
		output += fmt.Sprintf("  BuildDate: %s\n", version.BuildDate)
	}
	if version.GitCommit != "" {
		output += fmt.Sprintf("  GitCommit: %s\n", version.GitCommit)
	}
	if version.GitTreeState != "" {
		output += fmt.Sprintf("  GitTreeState: %s\n", version.GitTreeState)
	}
	if version.GitTag != "" {
		output += fmt.Sprintf("  GitTag: %s\n", version.GitTag)
	}
	if version.GoVersion != "" {
		output += fmt.Sprintf("  GoVersion: %s\n", version.GoVersion)
	}
	if version.Compiler != "" {
		output += fmt.Sprintf("  Compiler: %s\n", version.Compiler)
	}
	if version.Platform != "" {
		output += fmt.Sprintf("  Platform: %s\n", version.Platform)
	}
	if version.ExtraBuildInfo != "" {
		output += fmt.Sprintf("  ExtraBuildInfo: %s\n", version.ExtraBuildInfo)
	}
	if version.KustomizeVersion != "" {
		output += fmt.Sprintf("  Kustomize Version: %s\n", version.KustomizeVersion)
	}
	if version.HelmVersion != "" {
		output += fmt.Sprintf("  Helm Version: %s\n", version.HelmVersion)
	}
	if version.KubectlVersion != "" {
		output += fmt.Sprintf("  Kubectl Version: %s\n", version.KubectlVersion)
	}
	if version.JsonnetVersion != "" {
		output += fmt.Sprintf("  Jsonnet Version: %s\n", version.JsonnetVersion)
	}
	return output
}
