package commands

import (
	"context"
	"fmt"
	"syscall"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

// NewLoginCommand returns a new instance of `argocd login` command
func NewLoginCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		// clientConfig clientcmd.ClientConfig
		username string
	)
	var command = &cobra.Command{
		Use:   "login",
		Short: "Log in to Argo CD",
		Long:  "Log in to Argo CD",
		Run: func(c *cobra.Command, args []string) {
			password, err := terminal.ReadPassword(syscall.Stdin)
			errors.CheckError(err)

			//conf, err := clientConfig.ClientConfig()

			//errors.CheckError(err)
			conn, sessionIf := argocdclient.NewClientOrDie(clientOpts).NewSessionClientOrDie()
			defer util.Close(conn)

			sessionRequest := session.SessionRequest{
				Username: username,
				Password: string(password),
			}
			createdSession, err := sessionIf.Create(context.Background(), &sessionRequest)
			errors.CheckError(err)
			fmt.Printf("user %q logged in with token %q\n", createdSession.Token)

			// reach out to backend for /api/v1/sessions/create

			// namespace, wasSpecified, err := clientConfig.Namespace()
			// errors.CheckError(err)
			// // authenticate here
			// namespace := "default"
			// kubeclientset, err := kubernetes.NewForConfig(clientConfig)
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// configManager, err := util.NewConfigManager(clientset, namespace)
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// settings, err := configManager.GetSettings()
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// sessionManager := SessionManager{settings.ServerSignature}

			// // valid, _ := settings.LoginLocalUser....

			// // token := sessionManager.Create(username)
			// // fmt.Println("token = ", token)
		},
	}
	command.Flags().StringVar(&username, "username", "", "the username of an account to authenticate")
	// clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}
