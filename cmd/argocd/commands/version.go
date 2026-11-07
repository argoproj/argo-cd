package commands

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v3/common"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// coreModeServerVersion is reported as the server version in core mode, where the version of the Argo CD
// components running in the cluster cannot be determined by the CLI.
const coreModeServerVersion = "unavailable in core mode"

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
		Run: cli.WithSignalContext(func(cmd *cobra.Command, _ []string, _ context.CancelFunc) {
			ctx := cmd.Context()

			cv := common.GetVersion()
			switch output {
			case "yaml", "json":
				v := make(map[string]any)

				if short {
					v["client"] = map[string]string{common.CommandCLI: cv.Version}
				} else {
					v["client"] = cv
				}

				if !client {
					sv := resolveServerVersion(ctx, clientOpts, serverVersion, cmd)

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
					sv := resolveServerVersion(ctx, clientOpts, serverVersion, cmd)
					fmt.Fprint(cmd.OutOrStdout(), printServerVersion(sv, short || (output == "short")))
				}
			default:
				log.Fatalf("unknown output format: %s", output)
			}
		}),
	}
	versionCmd.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|short")
	versionCmd.Flags().BoolVar(&short, "short", false, "print just the version number")
	versionCmd.Flags().BoolVar(&client, "client", false, "client version only (no server required)")
	return &versionCmd
}

// resolveServerVersion returns the version of the Argo CD API server. In core mode there is no API server to ask:
// the CLI starts an in-process API server built from its own code, which would report the CLI's version as the
// server version. In that case a placeholder is returned instead, so the output does not claim a wrong version.
func resolveServerVersion(ctx context.Context, clientOpts *argocdclient.ClientOptions, serverVersion *version.VersionMessage, c *cobra.Command) *version.VersionMessage {
	core, err := headless.IsCoreMode(clientOpts)
	errors.CheckError(err)
	if core {
		return &version.VersionMessage{Version: coreModeServerVersion}
	}
	if serverVersion != nil {
		return serverVersion
	}
	return getServerVersion(ctx, clientOpts, c)
}

func getServerVersion(ctx context.Context, options *argocdclient.ClientOptions, c *cobra.Command) *version.VersionMessage {
	conn, versionIf := headless.NewClientOrDie(options, c).NewVersionClientOrDieWithContext(ctx)
	defer utilio.Close(conn)

	v, err := versionIf.Version(ctx, &empty.Empty{})
	errors.CheckError(err)

	return v
}

func printClientVersion(version *common.Version, short bool) string {
	output := fmt.Sprintf("%s: %s\n", common.CommandCLI, version)
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
