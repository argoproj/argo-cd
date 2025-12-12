package commands

import (
	ctx "context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/utils"
	"github.com/argoproj/argo-cd/v3/common"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

const DialTime = 30 * time.Second

// NewLogoutCommand returns a new instance of `argocd logout` command
func NewLogoutCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "logout CONTEXT",
		Short: "Log out from Argo CD",
		Long:  "Log out from Argo CD",
		Example: `# Logout from the active Argo CD context
# This can be helpful for security reasons or when you want to switch between different Argo CD contexts or accounts.
argocd logout CONTEXT

# Logout from a specific context named 'cd.argoproj.io'
argocd logout cd.argoproj.io
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

			tlsTestResult, err := grpc_util.TestTLS(context, DialTime)
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
			req, err := http.NewRequestWithContext(ctx.Background(), http.MethodPost, logoutURL, http.NoBody)
			errors.CheckError(err)
			cookie := &http.Cookie{
				Name:  common.AuthCookieName,
				Value: token,
			}
			req.AddCookie(cookie)

			_, err = client.Do(req)
			errors.CheckError(err)

			promptUtil := utils.NewPrompt(globalClientOpts.PromptsEnabled)

			canLogout := promptUtil.Confirm(fmt.Sprintf("Are you sure you want to log out from '%s'?", context))
			if canLogout {
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
			} else {
				log.Infof("Logout from '%s' cancelled", context)
			}
		},
	}
	return command
}
