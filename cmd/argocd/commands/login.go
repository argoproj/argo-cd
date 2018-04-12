package commands

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util"
	util_config "github.com/argoproj/argo-cd/util/config"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

// NewLoginCommand returns a new instance of `argocd login` command
func NewLoginCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		username string
		password string
	)
	var command = &cobra.Command{
		Use:   "login",
		Short: "Log in to Argo CD",
		Long:  "Log in to Argo CD",
		Run: func(c *cobra.Command, args []string) {
			for username == "" {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Username: ")
				usernameRaw, err := reader.ReadString('\n')
				if err != nil {
					log.Fatal(err)
				}
				username = strings.TrimSpace(usernameRaw)
			}
			for password == "" {
				fmt.Print("Password: ")
				passwordRaw, err := terminal.ReadPassword(syscall.Stdin)
				if err != nil {
					log.Fatal(err)
				}
				password = string(passwordRaw)
				if password == "" {
					fmt.Print("\n")
				}
			}

			conn, sessionIf := argocdclient.NewClientOrDie(clientOpts).NewSessionClientOrDie()
			defer util.Close(conn)

			sessionRequest := session.SessionRequest{
				Username: username,
				Password: password,
			}
			createdSession, err := sessionIf.Create(context.Background(), &sessionRequest)
			errors.CheckError(err)
			fmt.Printf("user %q logged in successfully\n", username)

			// now persist the new token
			localConfig, err := util_config.ReadLocalConfig()
			if err != nil {
				log.Fatal(err)
			}
			localConfig.Sessions[clientOpts.ServerAddr] = createdSession.Token
			err = util_config.WriteLocalConfig(localConfig)
			if err != nil {
				log.Fatal(err)
			}

		},
	}
	command.Flags().StringVar(&username, "username", "", "the username of an account to authenticate")
	return command
}
