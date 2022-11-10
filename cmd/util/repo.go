package util

import (
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type RepoOptions struct {
	Repo                           appsv1.Repository
	Upsert                         bool
	SshPrivateKeyPath              string
	InsecureIgnoreHostKey          bool
	InsecureSkipServerVerification bool
	TlsClientCertPath              string
	TlsClientCertKeyPath           string
	EnableLfs                      bool
	EnableOci                      bool
	GithubAppId                    int64
	GithubAppInstallationId        int64
	GithubAppPrivateKeyPath        string
	GitHubAppEnterpriseBaseURL     string
	Proxy                          string
}

func AddRepoFlags(command *cobra.Command, opts *RepoOptions) {
	command.Flags().StringVar(&opts.Repo.Type, "type", common.DefaultRepoType, "Type of the repository, \"git\" or \"helm\"")
	command.Flags().StringVar(&opts.Repo.Name, "name", "", "Name of the repository, mandatory for repositories of type helm")
	command.Flags().StringVar(&opts.Repo.Project, "project", "", "Project of the repository")
	command.Flags().StringVar(&opts.Repo.Username, "username", "", "Username to the repository")
	command.Flags().StringVar(&opts.Repo.Password, "password", "", "Password to the repository")
	command.Flags().StringVar(&opts.SshPrivateKeyPath, "ssh-private-key-path", "", "Path to the private ssh key (e.g. ~/.ssh/id_rsa)")
	command.Flags().StringVar(&opts.TlsClientCertPath, "tls-client-cert-path", "", "Path to the TLS client cert (must be PEM format)")
	command.Flags().StringVar(&opts.TlsClientCertKeyPath, "tls-client-cert-key-path", "", "Path to the TLS client cert's key path (must be PEM format)")
	command.Flags().BoolVar(&opts.InsecureIgnoreHostKey, "insecure-ignore-host-key", false, "Disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)")
	command.Flags().BoolVar(&opts.InsecureSkipServerVerification, "insecure-skip-server-verification", false, "Disables server certificate and host key checks")
	command.Flags().BoolVar(&opts.EnableLfs, "enable-lfs", false, "Enable git-lfs (Large File Support) on this repository")
	command.Flags().BoolVar(&opts.EnableOci, "enable-oci", false, "Enable helm-oci (Helm OCI-Based Repository)")
	command.Flags().Int64Var(&opts.GithubAppId, "github-app-id", 0, "ID of the GitHub Application")
	command.Flags().Int64Var(&opts.GithubAppInstallationId, "github-app-installation-id", 0, "Installation id of the GitHub Application")
	command.Flags().StringVar(&opts.GithubAppPrivateKeyPath, "github-app-private-key-path", "", "Private key of the GitHub Application")
	command.Flags().StringVar(&opts.GitHubAppEnterpriseBaseURL, "github-app-enterprise-base-url", "", "Base url to use when using GitHub Enterprise (e.g. https://ghe.example.com/api/v3")
	command.Flags().StringVar(&opts.Proxy, "proxy", "", "Use proxy to access repository")
}
