package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	accountpkg "github.com/argoproj/argo-cd/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
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
	command.AddCommand(NewAccountGetUserInfoCommand(clientOpts))
	command.AddCommand(NewAccountCanICommand(clientOpts))
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

func NewAccountGetUserInfoCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "get-user-info",
		Short: "Get user info",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			conn, client := argocdclient.NewClientOrDie(clientOpts).NewSessionClientOrDie()
			defer util.Close(conn)

			ctx := context.Background()
			response, err := client.GetUserInfo(ctx, &session.GetUserInfoRequest{})
			errors.CheckError(err)

			switch output {
			case "yaml":
				yamlBytes, err := yaml.Marshal(response)
				errors.CheckError(err)
				fmt.Println(string(yamlBytes))
			case "json":
				jsonBytes, err := json.MarshalIndent(response, "", "  ")
				errors.CheckError(err)
				fmt.Println(string(jsonBytes))
			case "":
				fmt.Printf("Logged In: %v\n", response.LoggedIn)
				if response.LoggedIn {
					fmt.Printf("Username: %s\n", response.Username)
					fmt.Printf("Issuer: %s\n", response.Iss)
					fmt.Printf("Groups: %v\n", strings.Join(response.Groups, ","))
				}
			default:
				log.Fatalf("Unknown output format: %s", output)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: yaml, json")
	return command
}

func NewAccountCanICommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "can-i ACTION RESOURCE SUBRESOURCE",
		Short: "Can I",
		Example: fmt.Sprintf(`
# Can I sync any app?
argocd account can-i sync applications '*'

# Can I update a project?
argocd account can-i update projects 'default'

# Can I create a cluster?
argocd account can-i create cluster '*'

Actions: %v
Resources: %v
`, rbacpolicy.Resources, rbacpolicy.Actions),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			conn, client := argocdclient.NewClientOrDie(clientOpts).NewAccountClientOrDie()
			defer util.Close(conn)

			ctx := context.Background()
			response, err := client.CanI(ctx, &accountpkg.CanIRequest{
				Action:      args[0],
				Resource:    args[1],
				Subresource: args[2],
			})
			errors.CheckError(err)
			fmt.Println(response.Value)
		},
	}
}
