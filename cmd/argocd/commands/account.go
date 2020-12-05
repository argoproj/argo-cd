package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	timeutil "github.com/argoproj/pkg/time"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	accountpkg "github.com/argoproj/argo-cd/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/localconfig"
	sessionutil "github.com/argoproj/argo-cd/util/session"
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
	command.AddCommand(NewAccountListCommand(clientOpts))
	command.AddCommand(NewAccountGenerateTokenCommand(clientOpts))
	command.AddCommand(NewAccountGetCommand(clientOpts))
	command.AddCommand(NewAccountDeleteTokenCommand(clientOpts))
	return command
}

func NewAccountUpdatePasswordCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		account         string
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
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, usrIf := acdClient.NewAccountClientOrDie()
			defer io.Close(conn)

			userInfo := getCurrentAccount(acdClient)

			if userInfo.Iss == sessionutil.SessionManagerClaimsIssuer && currentPassword == "" {
				fmt.Print("*** Enter current password: ")
				password, err := terminal.ReadPassword(int(os.Stdin.Fd()))
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
				Name:            account,
			}

			ctx := context.Background()
			_, err := usrIf.UpdatePassword(ctx, &updatePasswordRequest)
			errors.CheckError(err)
			fmt.Printf("Password updated\n")

			if account == "" || account == userInfo.Username {
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
			}
		},
	}

	command.Flags().StringVar(&currentPassword, "current-password", "", "current password you wish to change")
	command.Flags().StringVar(&newPassword, "new-password", "", "new password you want to update to")
	command.Flags().StringVar(&account, "account", "", "an account name that should be updated. Defaults to current user account")
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
			defer io.Close(conn)

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
argocd account can-i create clusters '*'

Actions: %v
Resources: %v
`, rbacpolicy.Actions, rbacpolicy.Resources),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			conn, client := argocdclient.NewClientOrDie(clientOpts).NewAccountClientOrDie()
			defer io.Close(conn)

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

func printAccountNames(accounts []*accountpkg.Account) {
	for _, p := range accounts {
		fmt.Println(p.Name)
	}
}

func printAccountsTable(items []*accountpkg.Account) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tENABLED\tCAPABILITIES\n")
	for _, a := range items {
		fmt.Fprintf(w, "%s\t%v\t%s\n", a.Name, a.Enabled, strings.Join(a.Capabilities, ", "))
	}
	_ = w.Flush()
}

func NewAccountListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List accounts",
		Example: "argocd account list",
		Run: func(c *cobra.Command, args []string) {

			conn, client := argocdclient.NewClientOrDie(clientOpts).NewAccountClientOrDie()
			defer io.Close(conn)

			ctx := context.Background()
			response, err := client.ListAccounts(ctx, &accountpkg.ListAccountRequest{})

			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(response.Items, output, false)
				errors.CheckError(err)
			case "name":
				printAccountNames(response.Items)
			case "wide", "":
				printAccountsTable(response.Items)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|name")
	return cmd
}

func getCurrentAccount(clientset argocdclient.Client) session.GetUserInfoResponse {
	conn, client := clientset.NewSessionClientOrDie()
	defer io.Close(conn)
	userInfo, err := client.GetUserInfo(context.Background(), &session.GetUserInfoRequest{})
	errors.CheckError(err)
	return *userInfo
}

func NewAccountGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output  string
		account string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get account details",
		Example: `# Get the currently logged in account details
argocd account get

# Get details for an account by name
argocd account get --account <account-name>`,
		Run: func(c *cobra.Command, args []string) {
			clientset := argocdclient.NewClientOrDie(clientOpts)

			if account == "" {
				account = getCurrentAccount(clientset).Username
			}

			conn, client := clientset.NewAccountClientOrDie()
			defer io.Close(conn)

			acc, err := client.GetAccount(context.Background(), &accountpkg.GetAccountRequest{Name: account})

			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(acc, output, true)
				errors.CheckError(err)
			case "name":
				fmt.Println(acc.Name)
			case "wide", "":
				printAccountDetails(acc)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|name")
	cmd.Flags().StringVarP(&account, "account", "a", "", "Account name. Defaults to the current account.")
	return cmd
}

func printAccountDetails(acc *accountpkg.Account) {
	fmt.Printf(printOpFmtStr, "Name:", acc.Name)
	fmt.Printf(printOpFmtStr, "Enabled:", strconv.FormatBool(acc.Enabled))
	fmt.Printf(printOpFmtStr, "Capabilities:", strings.Join(acc.Capabilities, ", "))
	fmt.Println("\nTokens:")
	if len(acc.Tokens) == 0 {
		fmt.Println("NONE")
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tISSUED AT\tEXPIRING AT\n")
		for _, t := range acc.Tokens {
			expiresAtFormatted := "never"
			if t.ExpiresAt > 0 {
				expiresAt := time.Unix(t.ExpiresAt, 0)
				expiresAtFormatted = expiresAt.Format(time.RFC3339)
				if expiresAt.Before(time.Now()) {
					expiresAtFormatted = fmt.Sprintf("%s (expired)", expiresAtFormatted)
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\n", t.Id, time.Unix(t.IssuedAt, 0).Format(time.RFC3339), expiresAtFormatted)
		}
		_ = w.Flush()
	}
}

func NewAccountGenerateTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		account   string
		expiresIn string
		id        string
	)
	cmd := &cobra.Command{
		Use:   "generate-token",
		Short: "Generate account token",
		Example: `# Generate token for the currently logged in account
argocd account generate-token

# Generate token for the account with the specified name
argocd account generate-token --account <account-name>`,
		Run: func(c *cobra.Command, args []string) {

			clientset := argocdclient.NewClientOrDie(clientOpts)
			conn, client := clientset.NewAccountClientOrDie()
			defer io.Close(conn)
			if account == "" {
				account = getCurrentAccount(clientset).Username
			}
			expiresIn, err := timeutil.ParseDuration(expiresIn)
			errors.CheckError(err)
			response, err := client.CreateToken(context.Background(), &accountpkg.CreateTokenRequest{
				Name:      account,
				ExpiresIn: int64(expiresIn.Seconds()),
				Id:        id,
			})
			errors.CheckError(err)
			fmt.Println(response.Token)
		},
	}
	cmd.Flags().StringVarP(&account, "account", "a", "", "Account name. Defaults to the current account.")
	cmd.Flags().StringVarP(&expiresIn, "expires-in", "e", "0s", "Duration before the token will expire. (Default: No expiration)")
	cmd.Flags().StringVar(&id, "id", "", "Optional token id. Fallback to uuid if not value specified.")
	return cmd
}

func NewAccountDeleteTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		account string
	)
	cmd := &cobra.Command{
		Use:   "delete-token",
		Short: "Deletes account token",
		Example: `# Delete token of the currently logged in account
argocd account delete-token ID

# Delete token of the account with the specified name
argocd account generate-token --account <account-name>`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			id := args[0]

			clientset := argocdclient.NewClientOrDie(clientOpts)
			conn, client := clientset.NewAccountClientOrDie()
			defer io.Close(conn)
			if account == "" {
				account = getCurrentAccount(clientset).Username
			}
			_, err := client.DeleteToken(context.Background(), &accountpkg.DeleteTokenRequest{Name: account, Id: id})
			errors.CheckError(err)
		},
	}
	cmd.Flags().StringVarP(&account, "account", "a", "", "Account name. Defaults to the current account.")
	return cmd
}
