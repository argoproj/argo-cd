package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/git"
)

// NewCommand returns the argocd-ssh-proxy command. ssh runs it as its ProxyCommand for SSH
// repositories that have a SOCKS5 proxy configured (see git.SSHCreds): it connects to the Git
// server through the proxy named by the ARGOCD_SSH_PROXY_URL environment variable and relays the
// connection over stdin and stdout. It writes nothing else to stdout, which carries the SSH
// connection; errors go to stderr, where ssh shows them.
func NewCommand() *cobra.Command {
	command := cobra.Command{
		Use:               common.CommandSSHProxy + " HOST PORT",
		Short:             "Argo CD SSH proxy command, connecting to a Git server through a SOCKS5 proxy",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				errors.CheckError(fmt.Errorf("expected 2 arguments (host and port), got %d", len(args)))
			}
			proxyURL := os.Getenv(git.SSHProxyURLEnv)
			if proxyURL == "" {
				errors.CheckError(fmt.Errorf("%s is not set", git.SSHProxyURLEnv))
			}
			errors.CheckError(git.ProxySSHConnection(c.Context(), proxyURL, args[0], args[1], os.Stdin, os.Stdout))
		},
	}

	return &command
}
