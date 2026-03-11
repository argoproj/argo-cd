package commands

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/utils"
	"github.com/argoproj/argo-cd/v3/common"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/cli"
	errutil "github.com/argoproj/argo-cd/v3/util/errors"
	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

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
			errutil.CheckError(err)
			if localCfg == nil {
				log.Fatalf("Nothing to logout from")
			}

			promptUtil := utils.NewPrompt(globalClientOpts.PromptsEnabled)

			canLogout := promptUtil.Confirm(fmt.Sprintf("Are you sure you want to log out from '%s'?", context))
			if canLogout {
				if tlsTestResult, err := grpc_util.TestTLS(context, common.BearerTokenTimeout); err != nil {
					log.Warnf("failed to check the TLS config settings for the server : %v.", err)
					globalClientOpts.PlainText = true
				} else {
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
				}

				scheme := "https"
				if globalClientOpts.PlainText {
					scheme = "http"
				}
				if res, err := revokeServerToken(scheme, context, localCfg.GetToken(context), globalClientOpts.Insecure); err != nil {
					log.Warnf("failed to invalidate token on server: %v.", err)
				} else {
					_ = res.Body.Close()
					if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusSeeOther {
						log.Warnf("server returned unexpected status code %d during logout", res.StatusCode)
					} else {
						log.Infof("token successfully invalidated on server")
					}
				}

				// Remove token from local config
				ok := localCfg.RemoveToken(context)
				if !ok {
					log.Fatalf("Context %s does not exist", context)
				}

				err = localconfig.ValidateLocalConfig(*localCfg)
				if err != nil {
					log.Fatalf("Error in logging out: %s", err)
				}
				err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
				errutil.CheckError(err)

				fmt.Printf("Logged out from '%s'\n", context)
			} else {
				log.Infof("Logout from '%s' cancelled", context)
			}
		},
	}
	return command
}

// revokeServerToken makes a call to the server logout endpoint to revoke the token server side
func revokeServerToken(scheme, hostName, token string, insecure bool) (res *http.Response, err error) {
	if token == "" {
		return nil, errors.New("error getting token from local context file")
	}
	logoutURL := fmt.Sprintf("%s://%s%s", scheme, hostName, common.LogoutEndpoint)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, logoutURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	cookie := &http.Cookie{
		Name:  common.AuthCookieName,
		Value: token,
	}
	req.AddCookie(cookie)

	client := &http.Client{Timeout: common.TokenRevocationClientTimeout}

	if insecure {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client.Transport = tr
	}
	return client.Do(req)
}
