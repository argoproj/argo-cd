package commands

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
	grpc_util "github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

// NewLogoutCommand returns a new instance of `argocd logout` command
func NewLogoutCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "logout CONTEXT",
		Short: "Log out from Argo CD",
		Long:  "Log out from Argo CD",
		Example: `# To log out of argocd
$ argocd logout
# This can be helpful for security reasons or when you want to switch between different Argo CD contexts or accounts.
`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			context := args[0]

			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				log.Fatalf("Nothing to logout from")
			}

			token := localCfg.GetToken(context)
			if token == "" {
				log.Fatalf("Error in getting token from context")
			}

			client := &http.Client{}

			dialTime := 30 * time.Second
			tlsTestResult, err := grpc_util.TestTLS(context, dialTime)
			errors.CheckError(err)
			if !tlsTestResult.TLS {
				if !globalClientOpts.PlainText {
					if !cli.AskToProceed("WARNING: server is not configured with TLS. Proceed (y/n)? ") {
						os.Exit(1)
					}
					globalClientOpts.PlainText = true
				}
			} else if tlsTestResult.InsecureErr != nil {
				if !globalClientOpts.Insecure {
					if !cli.AskToProceed(fmt.Sprintf("WARNING: server certificate had error: %s. Proceed insecurely (y/n)? ", tlsTestResult.InsecureErr)) {
						os.Exit(1)
					}
					globalClientOpts.Insecure = true
				}
			}

			scheme := "https"
			if globalClientOpts.PlainText {
				scheme = strings.TrimSuffix(scheme, "s")
			} else if globalClientOpts.Insecure {
				client.Transport = &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				}
			}

			logoutURL := fmt.Sprintf("%s://%s%s", scheme, context, common.LogoutEndpoint)
			req, err := http.NewRequest("POST", logoutURL, nil)
			errors.CheckError(err)
			cookie := &http.Cookie{
				Name:  common.AuthCookieName,
				Value: token,
			}
			req.AddCookie(cookie)

			_, err = client.Do(req)
			errors.CheckError(err)

			ok := localCfg.RemoveToken(context)
			if !ok {
				log.Fatalf("Context %s does not exist", context)
			}

			err = localconfig.ValidateLocalConfig(*localCfg)
			if err != nil {
				log.Fatalf("Error in logging out: %s", err)
			}

			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckError(err)

			fmt.Printf("Logged out from '%s'\n", context)
		},
	}
	return command
}
