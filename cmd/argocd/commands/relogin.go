package commands

import (
	"fmt"
	"os"

	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/localconfig"
	"github.com/argoproj/argo-cd/util/session"
)

// NewReloginCommand returns a new instance of `argocd relogin` command
func NewReloginCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		password string
	)
	var command = &cobra.Command{
		Use:   "relogin",
		Short: "Refresh an expired authenticate token",
		Long:  "Refresh an expired authenticate token",
		Run: func(c *cobra.Command, args []string) {
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

			parser := &jwt.Parser{
				SkipClaimsValidation: true,
			}
			claims := jwt.StandardClaims{}
			_, _, err = parser.ParseUnverified(configCtx.User.AuthToken, &claims)
			errors.CheckError(err)

			var tokenString string
			var refreshToken string
			if claims.Issuer == session.SessionManagerClaimsIssuer {
				clientOpts := argocdclient.ClientOptions{
					ConfigPath: "",
					ServerAddr: configCtx.Server.Server,
					Insecure:   configCtx.Server.Insecure,
					PlainText:  configCtx.Server.PlainText,
				}
				acdClient := argocdclient.NewClientOrDie(&clientOpts)
				fmt.Printf("Relogging in as '%s'\n", claims.Subject)
				tokenString = passwordLogin(acdClient, claims.Subject, password)
			} else {
				fmt.Println("Reinitiating SSO login")
				tokenString, refreshToken = oauth2Login(configCtx.Server.Server, configCtx.Server.PlainText)
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
	}
	command.Flags().StringVar(&password, "password", "", "the password of an account to authenticate")
	return command
}
