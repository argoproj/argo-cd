package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/git"
)

// NewRepoCommand returns a new instance of an `argocd repo` command
func NewRepoCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repo",
		Short: "Manage git repository connection parameters",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}

	command.AddCommand(NewRepoAddCommand(clientOpts))
	command.AddCommand(NewRepoListCommand(clientOpts))
	command.AddCommand(NewRepoRemoveCommand(clientOpts))
	return command
}

// NewRepoAddCommand returns a new instance of an `argocd repo add` command
func NewRepoAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		repo                           appsv1.Repository
		upsert                         bool
		sshPrivateKeyPath              string
		insecureIgnoreHostKey          bool
		insecureSkipServerVerification bool
		tlsClientCertPath              string
		tlsClientCertKeyPath           string
		enableLfs                      bool
	)

	// For better readability and easier formatting
	var repoAddExamples = `
Add a SSH repository using a private key for authentication, ignoring the server's host key:",
  $ argocd repo add git@git.example.com --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa",
Add a HTTPS repository using username/password and TLS client certificates:",
  $ argocd repo add https://git.example.com --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key",
Add a HTTPS repository using username/password without verifying the server's TLS certificate:",
  $ argocd repo add https://git.example.com --username git --password secret --insecure-skip-server-verification",
`

	var command = &cobra.Command{
		Use:     "add REPOURL",
		Short:   "Add git repository connection parameters",
		Example: repoAddExamples,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			// Repository URL
			repo.Repo = args[0]

			// Specifying ssh-private-key-path is only valid for SSH repositories
			if sshPrivateKeyPath != "" {
				if ok, _ := git.IsSSHURL(repo.Repo); ok {
					keyData, err := ioutil.ReadFile(sshPrivateKeyPath)
					if err != nil {
						log.Fatal(err)
					}
					repo.SSHPrivateKey = string(keyData)
				} else {
					err := fmt.Errorf("--ssh-private-key-path is only supported for SSH repositories.")
					errors.CheckError(err)
				}
			}

			// tls-client-cert-path and tls-client-cert-key-key-path must always be
			// specified together
			if (tlsClientCertPath != "" && tlsClientCertKeyPath == "") || (tlsClientCertPath == "" && tlsClientCertKeyPath != "") {
				err := fmt.Errorf("--tls-client-cert-path and --tls-client-cert-key-path must be specified together")
				errors.CheckError(err)
			}

			// Specifying tls-client-cert-path is only valid for HTTPS repositories
			if tlsClientCertPath != "" {
				if git.IsHTTPSURL(repo.Repo) {
					tlsCertData, err := ioutil.ReadFile(tlsClientCertPath)
					errors.CheckError(err)
					tlsCertKey, err := ioutil.ReadFile(tlsClientCertKeyPath)
					errors.CheckError(err)
					repo.TLSClientCertData = string(tlsCertData)
					repo.TLSClientCertKey = string(tlsCertKey)
				} else {
					err := fmt.Errorf("--tls-client-cert-path is only supported for HTTPS repositories")
					errors.CheckError(err)
				}
			}

			// InsecureIgnoreHostKey is deprecated and only here for backwards compat
			repo.InsecureIgnoreHostKey = insecureIgnoreHostKey
			repo.Insecure = insecureSkipServerVerification
			repo.EnableLFS = enableLfs

			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)

			// If the user set a username, but didn't supply password via --password,
			// then we prompt for it
			if repo.Username != "" && repo.Password == "" {
				repo.Password = cli.PromptPassword(repo.Password)
			}

			// We let the server check access to the repository before adding it. If
			// it is a private repo, but we cannot access with with the credentials
			// that were supplied, we bail out.
			repoAccessReq := repositorypkg.RepoAccessQuery{
				Repo:              repo.Repo,
				Username:          repo.Username,
				Password:          repo.Password,
				SshPrivateKey:     repo.SSHPrivateKey,
				TlsClientCertData: repo.TLSClientCertData,
				TlsClientCertKey:  repo.TLSClientCertKey,
				Insecure:          repo.IsInsecure(),
			}
			_, err := repoIf.ValidateAccess(context.Background(), &repoAccessReq)
			errors.CheckError(err)

			repoCreateReq := repositorypkg.RepoCreateRequest{
				Repo:   &repo,
				Upsert: upsert,
			}
			createdRepo, err := repoIf.Create(context.Background(), &repoCreateReq)
			errors.CheckError(err)
			fmt.Printf("repository '%s' added\n", createdRepo.Repo)
		},
	}
	command.Flags().StringVar(&repo.Username, "username", "", "username to the repository")
	command.Flags().StringVar(&repo.Password, "password", "", "password to the repository")
	command.Flags().StringVar(&sshPrivateKeyPath, "ssh-private-key-path", "", "path to the private ssh key (e.g. ~/.ssh/id_rsa)")
	command.Flags().StringVar(&tlsClientCertPath, "tls-client-cert-path", "", "path to the TLS client cert (must be PEM format)")
	command.Flags().StringVar(&tlsClientCertKeyPath, "tls-client-cert-key-path", "", "path to the TLS client cert's key path (must be PEM format)")
	command.Flags().BoolVar(&insecureIgnoreHostKey, "insecure-ignore-host-key", false, "disables SSH strict host key checking (deprecated, use --insecure-skip-server-validation instead)")
	command.Flags().BoolVar(&insecureSkipServerVerification, "insecure-skip-server-verification", false, "disables server certificate and host key checks")
	command.Flags().BoolVar(&enableLfs, "enable-lfs", false, "enable git-lfs (Large File Support) on this repository")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing repository with the same name even if the spec differs")
	return command
}

// NewRepoRemoveCommand returns a new instance of an `argocd repo list` command
func NewRepoRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm REPO",
		Short: "Remove git repository credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			for _, repoURL := range args {
				_, err := repoIf.Delete(context.Background(), &repositorypkg.RepoQuery{Repo: repoURL})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// Print table of repo info
func printRepoTable(repos []appsv1.Repository) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "REPO\tINSECURE\tLFS\tUSER\tSTATUS\tMESSAGE\n")
	for _, r := range repos {
		var username string
		if r.Username == "" {
			username = "-"
		} else {
			username = r.Username
		}
		fmt.Fprintf(w, "%s\t%v\t%v\t%s\t%s\t%s\n", r.Repo, r.IsInsecure(), r.EnableLFS, username, r.ConnectionState.Status, r.ConnectionState.Message)
	}
	_ = w.Flush()
}

// Print list of repo urls
func printRepoUrls(repos []appsv1.Repository) {
	for _, r := range repos {
		fmt.Println(r.Repo)
	}
}

// NewRepoListCommand returns a new instance of an `argocd repo rm` command
func NewRepoListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured repositories",
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			repos, err := repoIf.List(context.Background(), &repositorypkg.RepoQuery{})
			errors.CheckError(err)
			if output == "url" {
				printRepoUrls(repos.Items)
			} else {
				printRepoTable(repos.Items)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|url")
	return command
}
