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
		repo                  appsv1.Repository
		upsert                bool
		sshPrivateKeyPath     string
		insecureIgnoreHostKey bool
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
			repo.InsecureIgnoreHostKey = insecureIgnoreHostKey
			// First test the repo *without* username/password. This gives us a hint on whether this
			// is a private repo.
			// NOTE: it is important not to run git commands to test git credentials on the user's
			// system since it may mess with their git credential store (e.g. osx keychain).
			// See issue #315
			err := git.TestRepo(repo.Repo, "", "", repo.SSHPrivateKey, repo.InsecureIgnoreHostKey)
			if err != nil {
				if yes, _ := git.IsSSHURL(repo.Repo); yes {
					// If we failed using git SSH credentials, then the repo is automatically bad
					log.Fatal(err)
				}
				// If we can't test the repo, it's probably private. Prompt for credentials and
				// let the server test it.
				repo.Username, repo.Password = cli.PromptCredentials(repo.Username, repo.Password)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
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
	command.Flags().BoolVar(&insecureIgnoreHostKey, "insecure-ignore-host-key", false, "disables SSH strict host key checking")
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

// NewRepoListCommand returns a new instance of an `argocd repo rm` command
func NewRepoListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured repositories",
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			repos, err := repoIf.List(context.Background(), &repositorypkg.RepoQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "REPO\tUSER\tSTATUS\tMESSAGE\n")
			for _, r := range repos.Items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Repo, r.Username, r.ConnectionState.Status, r.ConnectionState.Message)
			}
			_ = w.Flush()
		},
	}
	return command
}
