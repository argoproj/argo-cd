package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	repocredspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repocreds"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/io"
)

// NewRepoCredsCommand returns a new instance of an `argocd repocreds` command
func NewRepoCredsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repocreds",
		Short: "Manage repository connection parameters",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}

	command.AddCommand(NewRepoCredsAddCommand(clientOpts))
	command.AddCommand(NewRepoCredsListCommand(clientOpts))
	command.AddCommand(NewRepoCredsRemoveCommand(clientOpts))
	return command
}

// NewRepoCredsAddCommand returns a new instance of an `argocd repocreds add` command
func NewRepoCredsAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		repo                    appsv1.RepoCreds
		upsert                  bool
		sshPrivateKeyPath       string
		tlsClientCertPath       string
		tlsClientCertKeyPath    string
		githubAppPrivateKeyPath string
	)

	// For better readability and easier formatting
	var repocredsAddExamples = `  # Add credentials with user/pass authentication to use for all repositories under https://git.example.com/repos
  argocd repocreds add https://git.example.com/repos/ --username git --password secret

  # Add credentials with SSH private key authentication to use for all repositories under ssh://git@git.example.com/repos
  argocd repocreds add ssh://git@git.example.com/repos/ --ssh-private-key-path ~/.ssh/id_rsa

  # Add credentials with GitHub App authentication to use for all repositories under https://github.com/repos
  argocd repocreds add https://github.com/repos/ --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem

  # Add credentials with GitHub App authentication to use for all repositories under https://ghe.example.com/repos
  argocd repocreds add https://ghe.example.com/repos/ --github-app-id 1 --github-app-installation-id 2 --github-app-private-key-path test.private-key.pem --github-app-enterprise-base-url https://ghe.example.com/api/v3

  # Add credentials with helm oci registry so that these oci registry urls do not need to be added as repos individually.
  argocd repocreds add localhost:5000/myrepo --enable-oci --type helm 
`

	var command = &cobra.Command{
		Use:     "add REPOURL",
		Short:   "Add git repository connection parameters",
		Example: repocredsAddExamples,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			// Repository URL
			repo.URL = args[0]

			// Specifying ssh-private-key-path is only valid for SSH repositories
			if sshPrivateKeyPath != "" {
				if ok, _ := git.IsSSHURL(repo.URL); ok {
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
				if git.IsHTTPSURL(repo.URL) {
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

			// Specifying github-app-private-key-path is only valid for HTTPS repositories
			if githubAppPrivateKeyPath != "" {
				if git.IsHTTPSURL(repo.URL) {
					githubAppPrivateKey, err := ioutil.ReadFile(githubAppPrivateKeyPath)
					errors.CheckError(err)
					repo.GithubAppPrivateKey = string(githubAppPrivateKey)
				} else {
					err := fmt.Errorf("--github-app-private-key-path is only supported for HTTPS repositories")
					errors.CheckError(err)
				}
			}

			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoCredsClientOrDie()
			defer io.Close(conn)

			// If the user set a username, but didn't supply password via --password,
			// then we prompt for it
			if repo.Username != "" && repo.Password == "" {
				repo.Password = cli.PromptPassword(repo.Password)
			}

			repoCreateReq := repocredspkg.RepoCredsCreateRequest{
				Creds:  &repo,
				Upsert: upsert,
			}

			createdRepo, err := repoIf.CreateRepositoryCredentials(context.Background(), &repoCreateReq)
			errors.CheckError(err)
			fmt.Printf("Repository credentials for '%s' added\n", createdRepo.URL)
		},
	}
	command.Flags().StringVar(&repo.Username, "username", "", "username to the repository")
	command.Flags().StringVar(&repo.Password, "password", "", "password to the repository")
	command.Flags().StringVar(&sshPrivateKeyPath, "ssh-private-key-path", "", "path to the private ssh key (e.g. ~/.ssh/id_rsa)")
	command.Flags().StringVar(&tlsClientCertPath, "tls-client-cert-path", "", "path to the TLS client cert (must be PEM format)")
	command.Flags().StringVar(&tlsClientCertKeyPath, "tls-client-cert-key-path", "", "path to the TLS client cert's key path (must be PEM format)")
	command.Flags().Int64Var(&repo.GithubAppId, "github-app-id", 0, "id of the GitHub Application")
	command.Flags().Int64Var(&repo.GithubAppInstallationId, "github-app-installation-id", 0, "installation id of the GitHub Application")
	command.Flags().StringVar(&githubAppPrivateKeyPath, "github-app-private-key-path", "", "private key of the GitHub Application")
	command.Flags().StringVar(&repo.GitHubAppEnterpriseBaseURL, "github-app-enterprise-base-url", "", "base url to use when using GitHub Enterprise (e.g. https://ghe.example.com/api/v3")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing repository with the same name even if the spec differs")
	command.Flags().BoolVar(&repo.EnableOCI, "enable-oci", false, "Specifies whether helm-oci support should be enabled for this repo")
	command.Flags().StringVar(&repo.Type, "type", common.DefaultRepoType, "type of the repository, \"git\" or \"helm\"")
	return command
}

// NewRepoCredsRemoveCommand returns a new instance of an `argocd repocreds rm` command
func NewRepoCredsRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm CREDSURL",
		Short: "Remove repository credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoCredsClientOrDie()
			defer io.Close(conn)
			for _, repoURL := range args {
				_, err := repoIf.DeleteRepositoryCredentials(context.Background(), &repocredspkg.RepoCredsDeleteRequest{Url: repoURL})
				errors.CheckError(err)
				fmt.Printf("Repository credentials for '%s' removed\n", repoURL)
			}
		},
	}
	return command
}

// Print the repository credentials as table
func printRepoCredsTable(repos []appsv1.RepoCreds) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "URL PATTERN\tUSERNAME\tSSH_CREDS\tTLS_CREDS\n")
	for _, r := range repos {
		if r.Username == "" {
			r.Username = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%v\n", r.URL, r.Username, r.SSHPrivateKey != "", r.TLSClientCertData != "")
	}
	_ = w.Flush()
}

// Print list of repo urls or url patterns for repository credentials
func printRepoCredsUrls(repos []appsv1.RepoCreds) {
	for _, r := range repos {
		fmt.Println(r.URL)
	}
}

// NewRepoCredsListCommand returns a new instance of an `argocd repo list` command
func NewRepoCredsListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured repository credentials",
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoCredsClientOrDie()
			defer io.Close(conn)
			repos, err := repoIf.ListRepositoryCredentials(context.Background(), &repocredspkg.RepoCredsQuery{})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(repos.Items, output, false)
				errors.CheckError(err)
			case "url":
				printRepoCredsUrls(repos.Items)
			case "wide", "":
				printRepoCredsTable(repos.Items)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|url")
	return command
}
