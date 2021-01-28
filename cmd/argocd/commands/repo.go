package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/io"
)

// NewRepoCommand returns a new instance of an `argocd repo` command
func NewRepoCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repo",
		Short: "Manage repository connection parameters",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}

	command.AddCommand(NewRepoAddCommand(clientOpts))
	command.AddCommand(NewRepoGetCommand(clientOpts))
	command.AddCommand(NewRepoListCommand(clientOpts))
	command.AddCommand(NewRepoRemoveCommand(clientOpts))
	return command
}

// NewRepoAddCommand returns a new instance of an `argocd repo add` command
func NewRepoAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		repoOpts cmdutil.RepoOptions
	)

	// For better readability and easier formatting
	var repoAddExamples = `  # Add a Git repository via SSH using a private key for authentication, ignoring the server's host key:
	argocd repo add git@git.example.com:repos/repo --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa

	# Add a Git repository via SSH on a non-default port - need to use ssh:// style URLs here
	argocd repo add ssh://git@git.example.com:2222/repos/repo --ssh-private-key-path ~/id_rsa

  # Add a private Git repository via HTTPS using username/password and TLS client certificates:
  argocd repo add https://git.example.com/repos/repo --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key

  # Add a private Git repository via HTTPS using username/password without verifying the server's TLS certificate
  argocd repo add https://git.example.com/repos/repo --username git --password secret --insecure-skip-server-verification

  # Add a public Helm repository named 'stable' via HTTPS
  argocd repo add https://kubernetes-charts.storage.googleapis.com --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd repo add https://kubernetes-charts.storage.googleapis.com --type helm --name stable --username test --password test

  # Add a private Helm OCI-based repository named 'stable' via HTTPS
  argocd repo add helm-oci-registry.cn-zhangjiakou.cr.aliyuncs.com --type helm --name stable --enable-oci --username test --password test
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
			repoOpts.Repo.Repo = args[0]

			// Specifying ssh-private-key-path is only valid for SSH repositories
			if repoOpts.SshPrivateKeyPath != "" {
				if ok, _ := git.IsSSHURL(repoOpts.Repo.Repo); ok {
					keyData, err := ioutil.ReadFile(repoOpts.SshPrivateKeyPath)
					if err != nil {
						log.Fatal(err)
					}
					repoOpts.Repo.SSHPrivateKey = string(keyData)
				} else {
					err := fmt.Errorf("--ssh-private-key-path is only supported for SSH repositories.")
					errors.CheckError(err)
				}
			}

			// tls-client-cert-path and tls-client-cert-key-key-path must always be
			// specified together
			if (repoOpts.TlsClientCertPath != "" && repoOpts.TlsClientCertKeyPath == "") || (repoOpts.TlsClientCertPath == "" && repoOpts.TlsClientCertKeyPath != "") {
				err := fmt.Errorf("--tls-client-cert-path and --tls-client-cert-key-path must be specified together")
				errors.CheckError(err)
			}

			// Specifying tls-client-cert-path is only valid for HTTPS repositories
			if repoOpts.TlsClientCertPath != "" {
				if git.IsHTTPSURL(repoOpts.Repo.Repo) {
					tlsCertData, err := ioutil.ReadFile(repoOpts.TlsClientCertPath)
					errors.CheckError(err)
					tlsCertKey, err := ioutil.ReadFile(repoOpts.TlsClientCertKeyPath)
					errors.CheckError(err)
					repoOpts.Repo.TLSClientCertData = string(tlsCertData)
					repoOpts.Repo.TLSClientCertKey = string(tlsCertKey)
				} else {
					err := fmt.Errorf("--tls-client-cert-path is only supported for HTTPS repositories")
					errors.CheckError(err)
				}
			}

			// Set repository connection properties only when creating repository, not
			// when creating repository credentials.
			// InsecureIgnoreHostKey is deprecated and only here for backwards compat
			repoOpts.Repo.InsecureIgnoreHostKey = repoOpts.InsecureIgnoreHostKey
			repoOpts.Repo.Insecure = repoOpts.InsecureSkipServerVerification
			repoOpts.Repo.EnableLFS = repoOpts.EnableLfs
			repoOpts.Repo.EnableOCI = repoOpts.EnableOci

			if repoOpts.Repo.Type == "helm" && repoOpts.Repo.Name == "" {
				errors.CheckError(fmt.Errorf("Must specify --name for repos of type 'helm'"))
			}

			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)

			// If the user set a username, but didn't supply password via --password,
			// then we prompt for it
			if repoOpts.Repo.Username != "" && repoOpts.Repo.Password == "" {
				repoOpts.Repo.Password = cli.PromptPassword(repoOpts.Repo.Password)
			}

			// We let the server check access to the repository before adding it. If
			// it is a private repo, but we cannot access with with the credentials
			// that were supplied, we bail out.
			//
			// Skip validation if we are just adding credentials template, chances
			// are high that we do not have the given URL pointing to a valid Git
			// repo anyway.
			repoAccessReq := repositorypkg.RepoAccessQuery{
				Repo:              repoOpts.Repo.Repo,
				Type:              repoOpts.Repo.Type,
				Name:              repoOpts.Repo.Name,
				Username:          repoOpts.Repo.Username,
				Password:          repoOpts.Repo.Password,
				SshPrivateKey:     repoOpts.Repo.SSHPrivateKey,
				TlsClientCertData: repoOpts.Repo.TLSClientCertData,
				TlsClientCertKey:  repoOpts.Repo.TLSClientCertKey,
				Insecure:          repoOpts.Repo.IsInsecure(),
				EnableOci:         repoOpts.Repo.EnableOCI,
			}
			_, err := repoIf.ValidateAccess(context.Background(), &repoAccessReq)
			errors.CheckError(err)

			repoCreateReq := repositorypkg.RepoCreateRequest{
				Repo:   &repoOpts.Repo,
				Upsert: repoOpts.Upsert,
			}

			createdRepo, err := repoIf.Create(context.Background(), &repoCreateReq)
			errors.CheckError(err)
			fmt.Printf("Repository '%s' added\n", createdRepo.Repo)
		},
	}
	command.Flags().BoolVar(&repoOpts.Upsert, "upsert", false, "Override an existing repository with the same name even if the spec differs")
	cmdutil.AddRepoFlags(command, &repoOpts)
	return command
}

// NewRepoRemoveCommand returns a new instance of an `argocd repo list` command
func NewRepoRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm REPO",
		Short: "Remove repository credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)
			for _, repoURL := range args {
				_, err := repoIf.Delete(context.Background(), &repositorypkg.RepoQuery{Repo: repoURL})
				errors.CheckError(err)
				fmt.Printf("Repository '%s' removed\n", repoURL)
			}
		},
	}
	return command
}

// Print table of repo info
func printRepoTable(repos appsv1.Repositories) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "TYPE\tNAME\tREPO\tINSECURE\tOCI\tLFS\tCREDS\tSTATUS\tMESSAGE\n")
	for _, r := range repos {
		var hasCreds string
		if !r.HasCredentials() {
			hasCreds = "false"
		} else {
			if r.InheritedCreds {
				hasCreds = "inherited"
			} else {
				hasCreds = "true"
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%v\t%v\t%s\t%s\t%s\n", r.Type, r.Name, r.Repo, r.IsInsecure(), r.EnableOCI, r.EnableLFS, hasCreds, r.ConnectionState.Status, r.ConnectionState.Message)
	}
	_ = w.Flush()
}

// Print list of repo urls or url patterns for repository credentials
func printRepoUrls(repos appsv1.Repositories) {
	for _, r := range repos {
		fmt.Println(r.Repo)
	}
}

// NewRepoListCommand returns a new instance of an `argocd repo rm` command
func NewRepoListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output  string
		refresh string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured repositories",
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)
			forceRefresh := false
			switch refresh {
			case "":
			case "hard":
				forceRefresh = true
			default:
				err := fmt.Errorf("--refresh must be one of: 'hard'")
				errors.CheckError(err)
			}
			repos, err := repoIf.List(context.Background(), &repositorypkg.RepoQuery{ForceRefresh: forceRefresh})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(repos.Items, output, false)
				errors.CheckError(err)
			case "url":
				printRepoUrls(repos.Items)
				// wide is the default
			case "wide", "":
				printRepoTable(repos.Items)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|url")
	command.Flags().StringVar(&refresh, "refresh", "", "Force a cache refresh on connection status")
	return command
}

// NewRepoGetCommand returns a new instance of an `argocd repo rm` command
func NewRepoGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output  string
		refresh string
	)
	var command = &cobra.Command{
		Use:   "get",
		Short: "Get a configured repository by URL",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			// Repository URL
			repoURL := args[0]
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)
			forceRefresh := false
			switch refresh {
			case "":
			case "hard":
				forceRefresh = true
			default:
				err := fmt.Errorf("--refresh must be one of: 'hard'")
				errors.CheckError(err)
			}
			repo, err := repoIf.Get(context.Background(), &repositorypkg.RepoQuery{Repo: repoURL, ForceRefresh: forceRefresh})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResource(repo, output)
				errors.CheckError(err)
			case "url":
				fmt.Println(repo.Repo)
				// wide is the default
			case "wide", "":
				printRepoTable(appsv1.Repositories{repo})
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|url")
	command.Flags().StringVar(&refresh, "refresh", "", "Force a cache refresh on connection status")
	return command
}
