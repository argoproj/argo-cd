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
)

// NewRepoCommand returns a new instance of an `argocd repo` command
func NewRepoCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repo",
		Short: "Manage git repository credentials",
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
	)
	var command = &cobra.Command{
		Use:   "add REPO",
		Short: "Add git repository credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			repo.Repo = args[0]
			if sshPrivateKeyPath != "" {
				keyData, err := ioutil.ReadFile(sshPrivateKeyPath)
				if err != nil {
					log.Fatal(err)
				}
				repo.SSHPrivateKey = string(keyData)
			}
			// InsecureIgnoreHostKey is deprecated and only here for backwards compat
			repo.InsecureIgnoreHostKey = insecureIgnoreHostKey
			repo.Insecure = insecureSkipServerVerification

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
				Repo:       repo.Repo,
				Username:   repo.Username,
				Password:   repo.Password,
				PrivateKey: repo.SSHPrivateKey,
				Insecure:   (repo.InsecureIgnoreHostKey || repo.Insecure),
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
	command.Flags().BoolVar(&insecureIgnoreHostKey, "insecure-ignore-host-key", false, "disables SSH strict host key checking (deprecated, use --insecure-skip-server-validation instead)")
	command.Flags().BoolVar(&insecureSkipServerVerification, "insecure-skip-server-verification", false, "disables server certificate and host key checks")
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
	fmt.Fprintf(w, "REPO\tINSECURE\tUSER\tSTATUS\tMESSAGE\n")
	for _, r := range repos {
		var username string
		if r.Username == "" {
			username = "-"
		} else {
			username = r.Username
		}
		fmt.Fprintf(w, "%s\t%v\t%s\t%s\t%s\n", r.Repo, r.Insecure, username, r.ConnectionState.Status, r.ConnectionState.Message)
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
