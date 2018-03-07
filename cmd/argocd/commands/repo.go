package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

// NewRepoCommand returns a new instance of an `argocd repo` command
func NewRepoCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repo",
		Short: fmt.Sprintf("%s repo COMMAND", cliName),
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
		repo appsv1.Repository
	)
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s repo add REPO", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			repo.Repo = args[0]
			err := git.TestRepo(repo.Repo, repo.Username, repo.Password)
			if err != nil {
				if repo.Username != "" && repo.Password != "" {
					// if everything was supplied, one of the inputs was definitely bad
					log.Fatal(err)
				}
				// If we can't test the repo, it's probably private. Prompt for credentials and try again.
				promptCredentials(&repo)
				err = git.TestRepo(repo.Repo, repo.Username, repo.Password)
			}
			errors.CheckError(err)
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			createdRepo, err := repoIf.Create(context.Background(), &repo)
			errors.CheckError(err)
			fmt.Printf("repository '%s' added\n", createdRepo.Repo)
		},
	}
	command.Flags().StringVar(&repo.Username, "username", "", "username to the repository")
	command.Flags().StringVar(&repo.Password, "password", "", "password to the repository")
	return command
}

func promptCredentials(repo *appsv1.Repository) {
	reader := bufio.NewReader(os.Stdin)
	if repo.Username == "" {
		fmt.Print("Username: ")
		username, _ := reader.ReadString('\n')
		repo.Username = username
	}
	if repo.Password == "" {
		fmt.Print("Password: ")
		bytePassword, _ := terminal.ReadPassword(syscall.Stdin)
		repo.Password = string(bytePassword)
	}
}

// NewRepoRemoveCommand returns a new instance of an `argocd repo list` command
func NewRepoRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm",
		Short: fmt.Sprintf("%s repo rm REPO", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			for _, repoURL := range args {
				_, err := repoIf.Delete(context.Background(), &repository.RepoQuery{Repo: repoURL})
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
		Short: fmt.Sprintf("%s repo list", cliName),
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			repos, err := repoIf.List(context.Background(), &repository.RepoQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "REPO\tUSER\n")
			for _, r := range repos.Items {
				fmt.Fprintf(w, "%s\t%s\n", r.Repo, r.Username)
			}
			_ = w.Flush()
		},
	}
	return command
}
