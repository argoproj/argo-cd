package commands

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	accountpkg "github.com/argoproj/argo-cd/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/localconfig"
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
				var err error
				newPassword, err = cli.ReadAndConfirmPassword()
				errors.CheckError(err)
			}

			updatePasswordRequest := accountpkg.UpdatePasswordRequest{
				NewPassword:     newPassword,
				CurrentPassword: currentPassword,
			}

			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, usrIf := acdClient.NewAccountClientOrDie()
			defer util.Close(conn)

			ctx := context.Background()
			_, err := usrIf.UpdatePassword(ctx, &updatePasswordRequest)
			errors.CheckError(err)
			fmt.Printf("Password updated\n")

			// Get a new JWT token after updating the password
			localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
			errors.CheckError(err)
			configCtx, err := localCfg.ResolveContext(clientOpts.Context)
			errors.CheckError(err)
			claims, err := configCtx.User.Claims()
			errors.CheckError(err)
			tokenString := passwordLogin(acdClient, claims.Subject, newPassword)
			localCfg.UpsertUser(localconfig.User{
				Name:      localCfg.CurrentContext,
				AuthToken: tokenString,
			})
			err = localconfig.WriteLocalConfig(*localCfg, clientOpts.ConfigPath)
			errors.CheckError(err)
			fmt.Printf("Context '%s' updated\n", localCfg.CurrentContext)
		},
	}

	command.Flags().StringVar(&currentPassword, "current-password", "", "current password you wish to change")
	command.Flags().StringVar(&newPassword, "new-password", "", "new password you want to update to")
	return command
}
