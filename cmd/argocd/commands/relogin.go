package commands

import (
	"fmt"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	settingspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
	"github.com/argoproj/argo-cd/v2/util/session"
)

// NewReloginCommand returns a new instance of `argocd relogin` command
func NewReloginCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		password         string
		ssoPort          int
		ssoLaunchBrowser bool
	)
	command := &cobra.Command{
		Use:   "relogin",
		Short: "Refresh an expired authenticate token",
		Long:  "Refresh an expired authenticate token",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				log.Fatalf("No context found. Login using `argocd login`")
			}
			configCtx, err := localCfg.ResolveContext(localCfg.CurrentContext)
			errors.CheckError(err)

			var tokenString string
			var refreshToken string
			clientOpts := argocdclient.ClientOptions{
				ConfigPath:        "",
				ServerAddr:        configCtx.Server.Server,
				Insecure:          configCtx.Server.Insecure,
				ClientCertFile:    globalClientOpts.ClientCertFile,
				ClientCertKeyFile: globalClientOpts.ClientCertKeyFile,
				GRPCWeb:           globalClientOpts.GRPCWeb,
				GRPCWebRootPath:   globalClientOpts.GRPCWebRootPath,
				PlainText:         configCtx.Server.PlainText,
				Headers:           globalClientOpts.Headers,
			}
			acdClient := headless.NewClientOrDie(&clientOpts, c)
			claims, err := configCtx.User.Claims()
			errors.CheckError(err)
			if claims.Issuer == session.SessionManagerClaimsIssuer {
				fmt.Printf("Relogging in as '%s'\n", localconfig.GetUsername(claims.Subject))
				tokenString = passwordLogin(ctx, acdClient, localconfig.GetUsername(claims.Subject), password)
			} else {
				fmt.Println("Reinitiating SSO login")
				setConn, setIf := acdClient.NewSettingsClientOrDie()
				defer argoio.Close(setConn)
				httpClient, err := acdClient.HTTPClient()
				errors.CheckError(err)
				ctx = oidc.ClientContext(ctx, httpClient)
				acdSet, err := setIf.Get(ctx, &settingspkg.SettingsQuery{})
				errors.CheckError(err)
				oauth2conf, provider, err := acdClient.OIDCConfig(ctx, acdSet)
				errors.CheckError(err)
				tokenString, refreshToken = oauth2Login(ctx, ssoPort, acdSet.GetOIDCConfig(), oauth2conf, provider, ssoLaunchBrowser)
			}

			localCfg.UpsertUser(localconfig.User{
				Name:         localCfg.CurrentContext,
				AuthToken:    tokenString,
				RefreshToken: refreshToken,
			})
			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckError(err)
			fmt.Printf("Context '%s' updated\n", localCfg.CurrentContext)
		},
		Example: `  
# Reinitiates the login with previous contexts
argocd relogin

# Reinitiates the login with password
argocd relogin --password YOUR_PASSWORD

# Configure direct access using Kubernetes API server
argocd login cd.argoproj.io --core

# If user logged in with - "argocd login cd.argoproj.io" with sso login
# The command - "argocd relogin" will Reinitiates SSO login and updates the server context`,
	}
	command.Flags().StringVar(&password, "password", "", "The password of an account to authenticate")
	command.Flags().IntVar(&ssoPort, "sso-port", DefaultSSOLocalPort, "Port to run local OAuth2 login application")
	command.Flags().BoolVar(&ssoLaunchBrowser, "sso-launch-browser", true, "Automatically launch the default browser when performing SSO login")
	return command
}
