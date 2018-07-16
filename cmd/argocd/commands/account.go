package commands

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/account"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

func NewAccountCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "account",
		Short: "Manage account settings",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewAccountUpdatePasswordCommand(clientOpts))
	return command
}

func NewAccountUpdatePasswordCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		currentPassword string
		newPassword     string
	)
	var command = &cobra.Command{
		Use:   "update-password",
		Short: "Update password",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			if currentPassword == "" {
				fmt.Print("*** Enter current password: ")
				password, err := terminal.ReadPassword(syscall.Stdin)
				errors.CheckError(err)
				currentPassword = string(password)
				fmt.Print("\n")
			}
			if newPassword == "" {
				newPassword = settings.ReadAndConfirmPassword()
			}

			updatePasswordRequest := account.UpdatePasswordRequest{
				NewPassword:     newPassword,
				CurrentPassword: currentPassword,
			}

			conn, usrIf := argocdclient.NewClientOrDie(clientOpts).NewAccountClientOrDie()
			defer util.Close(conn)
			_, err := usrIf.UpdatePassword(context.Background(), &updatePasswordRequest)
			errors.CheckError(err)
			fmt.Printf("Password updated\n")
		},
	}

	command.Flags().StringVar(&currentPassword, "current-password", "", "current password you wish to change")
	command.Flags().StringVar(&newPassword, "new-password", "", "new password you want to update to")
	return command
}
