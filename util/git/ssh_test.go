package git

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSHProxyGitSSHCommandInjection(t *testing.T) {
	proxyURL := "socks5://evil'$(id>argocd-ssh-proxy-poc-id)'host:1080"
	parsed, err := url.Parse(proxyURL)
	require.NoError(t, err)
	require.Equal(t, "evil'$(id>argocd-ssh-proxy-poc-id)'host", parsed.Hostname())

	creds := NewSSHCreds("sshPrivateKey", "", false, proxyURL)
	// The malicious proxy must have been stripped at construction time.
	require.Empty(t, creds.proxy, "malicious proxy must not be stored on the creds struct")

	closer, env, err := creds.Environ()
	require.NoError(t, err)
	defer closer.Close()

	var gitSSHCommand string
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_SSH_COMMAND=") {
			gitSSHCommand = entry
			break
		}
	}
	require.NotContains(t, gitSSHCommand, "ProxyCommand",
		"GIT_SSH_COMMAND must not contain a ProxyCommand built from a rejected proxy")
	require.NotContains(t, gitSSHCommand, "argocd-ssh-proxy-poc-id",
		"GIT_SSH_COMMAND must not echo any part of the malicious proxy string")
}
