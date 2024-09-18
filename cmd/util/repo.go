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
	NoProxy                        string
	GCPServiceAccountKeyPath       string
	ForceHttpBasicAuth             bool
}

func AddRepoFlags(command *cobra.Command, opts *RepoOptions) {
	command.Flags().StringVar(&opts.Repo.Type, "type", common.DefaultRepoType, "type of the repository, \"git\" or \"helm\"")
	command.Flags().StringVar(&opts.Repo.Name, "name", "", "name of the repository, mandatory for repositories of type helm")
	command.Flags().StringVar(&opts.Repo.Project, "project", "", "project of the repository")
	command.Flags().StringVar(&opts.Repo.Username, "username", "", "username to the repository")
	command.Flags().StringVar(&opts.Repo.Password, "password", "", "password to the repository")
	command.Flags().StringVar(&opts.SshPrivateKeyPath, "ssh-private-key-path", "", "path to the private ssh key (e.g. ~/.ssh/id_rsa)")
	command.Flags().StringVar(&opts.TlsClientCertPath, "tls-client-cert-path", "", "path to the TLS client cert (must be PEM format)")
	command.Flags().StringVar(&opts.TlsClientCertKeyPath, "tls-client-cert-key-path", "", "path to the TLS client cert's key path (must be PEM format)")
	command.Flags().BoolVar(&opts.InsecureIgnoreHostKey, "insecure-ignore-host-key", false, "disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)")
	command.Flags().BoolVar(&opts.InsecureSkipServerVerification, "insecure-skip-server-verification", false, "disables server certificate and host key checks")
	command.Flags().BoolVar(&opts.EnableLfs, "enable-lfs", false, "enable git-lfs (Large File Support) on this repository")
	command.Flags().BoolVar(&opts.EnableOci, "enable-oci", false, "enable helm-oci (Helm OCI-Based Repository)")
	command.Flags().Int64Var(&opts.GithubAppId, "github-app-id", 0, "id of the GitHub Application")
	command.Flags().Int64Var(&opts.GithubAppInstallationId, "github-app-installation-id", 0, "installation id of the GitHub Application")
	command.Flags().StringVar(&opts.GithubAppPrivateKeyPath, "github-app-private-key-path", "", "private key of the GitHub Application")
	command.Flags().StringVar(&opts.GitHubAppEnterpriseBaseURL, "github-app-enterprise-base-url", "", "base url to use when using GitHub Enterprise (e.g. https://ghe.example.com/api/v3")
	command.Flags().StringVar(&opts.Proxy, "proxy", "", "use proxy to access repository")
	command.Flags().StringVar(&opts.Proxy, "no-proxy", "", "don't access these targets via proxy")
	command.Flags().StringVar(&opts.GCPServiceAccountKeyPath, "gcp-service-account-key-path", "", "service account key for the Google Cloud Platform")
	command.Flags().BoolVar(&opts.ForceHttpBasicAuth, "force-http-basic-auth", false, "whether to force use of basic auth when connecting repository via HTTP")
}
