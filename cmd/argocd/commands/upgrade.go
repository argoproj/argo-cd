package commands

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/upgrade"
	"github.com/argoproj/argo-cd/v3/common"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// NewUpgradeCmd returns a new `upgrade` command to be used as a sub-command to root
func NewUpgradeCmd(
	clientOpts *argocdclient.ClientOptions,
	upgradeVersionTag string,
	currentVersionTag string,
	skipChecks bool,
) *cobra.Command {
	var (
		argUpgradeTag string
		err           error
	)

	// use `argocd`	cli version by default, or override with passed in value
	if upgradeVersionTag == "" {
		upgradeVersionTag = common.GetVersion().String()
	}

	upgradeCmd := cobra.Command{
		Use:   "upgrade",
		Short: "Check configuration for changes required before upgrading",

		Example: fmt.Sprintf(`  # Show changes for upgrading to the current argocd cli version
  %s upgrade

  # Show changes for a specific release tag. See: https://github.com/argoproj/argo-cd/releases
  %s upgrade --upgrade-tag v3.0.0
`, cliName, cliName),

		Run: func(cmd *cobra.Command, _ []string) {
			u := &upgrade.Upgrade{}

			// use Argo CD Server API version by default, or override with passed in value
			if currentVersionTag == "" {
				currentVersionTag = getServerVersion(cmd.Context(), clientOpts, cmd).Version
			}

			if argUpgradeTag != "" {
				upgradeVersionTag = argUpgradeTag
			}

			err = u.SetCurrentVersion(currentVersionTag)
			errors.CheckError(err)
			err = u.SetUpgradeVersion(upgradeVersionTag)
			errors.CheckError(err)

			cv := u.GetCurrentVersion()
			uv := u.GetUpgradeVersion()

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Performing checks for upgrade from %s to %s...\n",
				cv.SemVer, uv.SemVer)
			errors.CheckError(err)

			if !skipChecks {
				loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
				configOverrides := &clientcmd.ConfigOverrides{}
				clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
				err = upgrade.Run(u, clientConfig)
				errors.CheckError(err)
			}
		},
	}

	upgradeCmd.Flags().StringVar(
		&argUpgradeTag, "upgrade-tag",
		upgradeVersionTag,
		"Release tag to check for upgrades. See: https://github.com/argoproj/argo-cd/releases")
	return &upgradeCmd
}
