package util

import (
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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
}

func AddRepoFlags(command *cobra.Command, opts *RepoOptions) {
	command.Flags().StringVar(&opts.Repo.Type, "type", common.DefaultRepoType, "type of the repository, \"git\" or \"helm\"")
	command.Flags().StringVar(&opts.Repo.Name, "name", "", "name of the repository, mandatory for repositories of type helm")
	command.Flags().StringVar(&opts.Repo.Username, "username", "", "username to the repository")
	command.Flags().StringVar(&opts.Repo.Password, "password", "", "password to the repository")
	command.Flags().StringVar(&opts.SshPrivateKeyPath, "ssh-private-key-path", "", "path to the private ssh key (e.g. ~/.ssh/id_rsa)")
	command.Flags().StringVar(&opts.TlsClientCertPath, "tls-client-cert-path", "", "path to the TLS client cert (must be PEM format)")
	command.Flags().StringVar(&opts.TlsClientCertKeyPath, "tls-client-cert-key-path", "", "path to the TLS client cert's key path (must be PEM format)")
	command.Flags().BoolVar(&opts.InsecureIgnoreHostKey, "insecure-ignore-host-key", false, "disables SSH strict host key checking (deprecated, use --insecure-skip-server-verification instead)")
	command.Flags().BoolVar(&opts.InsecureSkipServerVerification, "insecure-skip-server-verification", false, "disables server certificate and host key checks")
	command.Flags().BoolVar(&opts.EnableLfs, "enable-lfs", false, "enable git-lfs (Large File Support) on this repository")
	command.Flags().BoolVar(&opts.EnableOci, "enable-oci", false, "enable helm-oci (Helm OCI-Based Repository)")
}
