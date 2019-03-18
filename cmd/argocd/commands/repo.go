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
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/repos"
)

// NewRepoCommand returns a new instance of an `argocd repo` command
func NewRepoCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repo",
		Short: "Manage repository credentials",
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
		caFile                string
		certFile              string
		keyFile               string
		repoType              string
		insecureIgnoreHostKey bool
	)
	var command = &cobra.Command{
		Use:   "add REPO",
		Short: "Add repository credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			repo.Repo = args[0]
			repo.Type = appsv1.RepoType(repoType)
			if sshPrivateKeyPath != "" {
				keyData, err := ioutil.ReadFile(sshPrivateKeyPath)
				if err != nil {
					log.Fatal(err)
				}
				repo.SSHPrivateKey = string(keyData)
			}
			repo.InsecureIgnoreHostKey = insecureIgnoreHostKey
			if caFile != "" {
				keyData, err := ioutil.ReadFile(caFile)
				if err != nil {
					log.Fatal(err)
				}
				repo.CAData = keyData
			}
			if certFile != "" {
				keyData, err := ioutil.ReadFile(certFile)
				if err != nil {
					log.Fatal(err)
				}
				repo.CertData = keyData
			}
			if keyFile != "" {
				keyData, err := ioutil.ReadFile(keyFile)
				if err != nil {
					log.Fatal(err)
				}
				repo.KeyData = keyData
			}
			// First test the repo *without* username/password. This gives us a hint on whether this
			// is a private repo.
			// NOTE: it is important not to run git commands to test git credentials on the user's
			// system since it may mess with their git credential store (e.g. osx keychain).
			// See issue #315
			config := repos.Config{Url: repo.Repo, Type: repoType, Name: repo.Name, SSHPrivateKey: repo.SSHPrivateKey, InsecureIgnoreHostKey: repo.InsecureIgnoreHostKey}
			err := repos.TestRepo(config)
			if err != nil {
				if repos.IsSSHURL(repo.Repo) {
					// If we failed using git SSH credentials, then the repo is automatically bad
					log.Fatal(err)
				}
				// If we can't test the repo, it's probably private. Prompt for credentials and
				// let the server test it.
				repo.Username, repo.Password = cli.PromptCredentials(repo.Username, repo.Password)
			}
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			repoCreateReq := repository.RepoCreateRequest{
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
	command.Flags().StringVar(&caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	command.Flags().StringVar(&certFile, "cert-file", "", "identify HTTPS client using this SSL certificate file")
	command.Flags().StringVar(&keyFile, "key-file", "", "identify HTTPS client using this SSL key file")
	command.Flags().StringVar(&repo.Name, "name", "", "the name of the the repo")
	command.Flags().StringVar(&repoType, "type", "git", "the type of the the repo (default is git)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing repository with the same name even if the spec differs")
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
		Short: "List configured repositories",
		Run: func(c *cobra.Command, args []string) {
			conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer util.Close(conn)
			repos, err := repoIf.List(context.Background(), &repository.RepoQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, err = fmt.Fprintf(w, "REPO\tTYPE\tNAME\tUSER\tSTATUS\tMESSAGE\n")
			errors.CheckError(err)
			for _, r := range repos.Items {
				_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Repo, r.Type, r.Name, r.Username, r.ConnectionState.Status, r.ConnectionState.Message)
				errors.CheckError(err)
			}
			_ = w.Flush()
		},
	}
	return command
}
