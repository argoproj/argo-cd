package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/localconfig"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

// NewLoginCommand returns a new instance of `argocd login` command
func NewLoginCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		username string
		password string
	)
	var command = &cobra.Command{
		Use:   "login SERVER",
		Short: "Log in to Argo CD",
		Long:  "Log in to Argo CD",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			server := args[0]
			tlsTestResult, err := grpc_util.TestTLS(server)
			errors.CheckError(err)
			if !tlsTestResult.TLS {
				if !globalClientOpts.PlainText {
					askToProceed("WARNING: server is not configured with TLS. Proceed (y/n)? ")
					globalClientOpts.PlainText = true
				}
			} else if tlsTestResult.InsecureErr != nil {
				if !globalClientOpts.Insecure {
					askToProceed(fmt.Sprintf("WARNING: server certificate had error: %s. Proceed insecurely (y/n)? ", tlsTestResult.InsecureErr))
					globalClientOpts.Insecure = true
				}
			}

			for username == "" {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Username: ")
				usernameRaw, err := reader.ReadString('\n')
				errors.CheckError(err)
				username = strings.TrimSpace(usernameRaw)
			}
			for password == "" {
				fmt.Print("Password: ")
				passwordRaw, err := terminal.ReadPassword(syscall.Stdin)
				errors.CheckError(err)
				password = string(passwordRaw)
				if password == "" {
					fmt.Print("\n")
				}
			}

			clientOpts := argocdclient.ClientOptions{
				ConfigPath: "",
				ServerAddr: server,
				Insecure:   globalClientOpts.Insecure,
				PlainText:  globalClientOpts.PlainText,
			}
			conn, sessionIf := argocdclient.NewClientOrDie(&clientOpts).NewSessionClientOrDie()
			defer util.Close(conn)

			sessionRequest := session.SessionCreateRequest{
				Username: username,
				Password: password,
			}
			createdSession, err := sessionIf.Create(context.Background(), &sessionRequest)
			errors.CheckError(err)
			fmt.Printf("user %q logged in successfully\n", username)

			// login successful. Persist the config
			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				localCfg = &localconfig.LocalConfig{}
			}
			localCfg.CurrentContext = server
			localCfg.UpsertServer(localconfig.Server{
				Server:    server,
				PlainText: globalClientOpts.PlainText,
				Insecure:  globalClientOpts.Insecure,
			})
			localCfg.UpsertUser(localconfig.User{
				Name:      server,
				AuthToken: createdSession.Token,
			})
			localCfg.UpsertContext(localconfig.ContextRef{
				Name:   server,
				User:   server,
				Server: server,
			})

			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckError(err)

		},
	}
	command.Flags().StringVar(&username, "username", "", "the username of an account to authenticate")
	command.Flags().StringVar(&password, "password", "", "the password of an account to authenticate")
	return command
}

func askToProceed(message string) {
	proceed := ""
	acceptedAnswers := map[string]bool{
		"y":   true,
		"yes": true,
		"n":   true,
		"no":  true,
	}
	for !acceptedAnswers[proceed] {
		fmt.Print(message)
		reader := bufio.NewReader(os.Stdin)
		proceedRaw, err := reader.ReadString('\n')
		errors.CheckError(err)
		proceed = strings.TrimSpace(proceedRaw)
	}
	if proceed == "no" || proceed == "n" {
		os.Exit(1)
	}
}
