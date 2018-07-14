package commands

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/users"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

func NewUsersCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "users",
		Short: "Manage users",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewUsersChangePasswordCommand(clientOpts))
	return command
}

func NewUsersChangePasswordCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		CurrentPassword string
		NewPassword     string
	)
	var command = &cobra.Command{
		Use:   "change-password USERNAME",
		Short: "Change User Password",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			if CurrentPassword == "" {
				fmt.Print("*** Enter your Current Password password ")
				password, err := terminal.ReadPassword(syscall.Stdin)
				errors.CheckError(err)
				CurrentPassword = string(password)
				fmt.Print("\n")
			}
			if NewPassword == "" {
				NewPassword = settings.ReadAndConfirmPassword()
			}

			userName := args[0]
			body := users.Body{
				CurrentPassword: CurrentPassword,
				NewPassword:     NewPassword,
			}
			UpdatePasswordRequest := users.UpdatePasswordRequest{
				Name: userName,
				Body: &body,
			}

			conn, usrIf := argocdclient.NewClientOrDie(clientOpts).NewUsersClientOrDie()
			defer util.Close(conn)
			_, err := usrIf.UpdatePassword(context.Background(), &UpdatePasswordRequest)
			errors.CheckError(err)
			fmt.Printf("password for user %s updated to %s \n", userName, NewPassword)
		},
	}

	command.Flags().StringVar(&CurrentPassword, "current-password", "", "current password you wish to change")
	command.Flags().StringVar(&NewPassword, "new-password", "", "new password you want to update to")
	return command
}
