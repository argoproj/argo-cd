package commands

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/util/errors"
	argoio "github.com/argoproj/argo-cd/util/io"
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
					sv := getServerVersion(clientOpts)

					if short {
						v["server"] = map[string]string{"argocd-server": sv.Version}
					} else {
						v["server"] = sv
					}
				}

				err := PrintResource(v, output)
				errors.CheckError(err)
			case "wide", "short", "":
				printClientVersion(&cv, short || (output == "short"))

				if !client {
					sv := getServerVersion(clientOpts)
					printServerVersion(sv, short || (output == "short"))
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

func getServerVersion(options *argocdclient.ClientOptions) *version.VersionMessage {
	conn, versionIf := argocdclient.NewClientOrDie(options).NewVersionClientOrDie()
	defer argoio.Close(conn)

	v, err := versionIf.Version(context.Background(), &empty.Empty{})
	errors.CheckError(err)

	return v
}

func printClientVersion(version *common.Version, short bool) {
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
}

func printServerVersion(version *version.VersionMessage, short bool) {
	fmt.Printf("%s: %s\n", "argocd-server", version.Version)

	if short {
		return
	}

	if version.BuildDate != "" {
		fmt.Printf("  BuildDate: %s\n", version.BuildDate)
	}
	if version.GitCommit != "" {
		fmt.Printf("  GitCommit: %s\n", version.GitCommit)
	}
	if version.GitTreeState != "" {
		fmt.Printf("  GitTreeState: %s\n", version.GitTreeState)
	}
	if version.GitTag != "" {
		fmt.Printf("  GitTag: %s\n", version.GitTag)
	}
	if version.GoVersion != "" {
		fmt.Printf("  GoVersion: %s\n", version.GoVersion)
	}
	if version.Compiler != "" {
		fmt.Printf("  Compiler: %s\n", version.Compiler)
	}
	if version.Platform != "" {
		fmt.Printf("  Platform: %s\n", version.Platform)
	}
	if version.KsonnetVersion != "" {
		fmt.Printf("  Ksonnet Version: %s\n", version.KsonnetVersion)
	}
	if version.KustomizeVersion != "" {
		fmt.Printf("  Kustomize Version: %s\n", version.KustomizeVersion)
	}
	if version.HelmVersion != "" {
		fmt.Printf("  Helm Version: %s\n", version.HelmVersion)
	}
	if version.KubectlVersion != "" {
		fmt.Printf("  Kubectl Version: %s\n", version.KubectlVersion)
	}
	if version.JsonnetVersion != "" {
		fmt.Printf("  Jsonnet Version: %s\n", version.JsonnetVersion)
	}
}
