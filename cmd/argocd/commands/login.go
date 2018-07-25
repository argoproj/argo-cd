package commands

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/server/settings"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/localconfig"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// NewLoginCommand returns a new instance of `argocd login` command
func NewLoginCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		ctxName  string
		username string
		password string
		sso      bool
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
			clientOpts := argocdclient.ClientOptions{
				ConfigPath: "",
				ServerAddr: server,
				Insecure:   globalClientOpts.Insecure,
				PlainText:  globalClientOpts.PlainText,
			}
			acdClient := argocdclient.NewClientOrDie(&clientOpts)
			setConn, setIf := acdClient.NewSettingsClientOrDie()
			defer util.Close(setConn)

			if ctxName == "" {
				ctxName = server
			}

			// Perform the login
			var tokenString string
			var refreshToken string
			if !sso {
				tokenString = passwordLogin(acdClient, username, password)
			} else {
				acdSet, err := setIf.Get(context.Background(), &settings.SettingsQuery{})
				errors.CheckError(err)
				if !ssoConfigured(acdSet) {
					log.Fatalf("ArgoCD instance is not configured with SSO")
				}
				tokenString, refreshToken = oauth2Login(server, clientOpts.PlainText)
			}

			parser := &jwt.Parser{
				SkipClaimsValidation: true,
			}
			claims := jwt.MapClaims{}
			_, _, err = parser.ParseUnverified(tokenString, &claims)
			errors.CheckError(err)

			fmt.Printf("'%s' logged in successfully\n", userDisplayName(claims))
			// login successful. Persist the config
			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				localCfg = &localconfig.LocalConfig{}
			}
			localCfg.UpsertServer(localconfig.Server{
				Server:    server,
				PlainText: globalClientOpts.PlainText,
				Insecure:  globalClientOpts.Insecure,
			})
			localCfg.UpsertUser(localconfig.User{
				Name:         ctxName,
				AuthToken:    tokenString,
				RefreshToken: refreshToken,
			})
			if ctxName == "" {
				ctxName = server
			}
			localCfg.CurrentContext = ctxName
			localCfg.UpsertContext(localconfig.ContextRef{
				Name:   ctxName,
				User:   ctxName,
				Server: server,
			})
			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckError(err)
			fmt.Printf("Context '%s' updated\n", ctxName)
		},
	}
	command.Flags().StringVar(&ctxName, "name", "", "name to use for the context")
	command.Flags().StringVar(&username, "username", "", "the username of an account to authenticate")
	command.Flags().StringVar(&password, "password", "", "the password of an account to authenticate")
	command.Flags().BoolVar(&sso, "sso", false, "Perform SSO login")
	return command
}

func userDisplayName(claims jwt.MapClaims) string {
	if email, ok := claims["email"]; ok && email != nil {
		return email.(string)
	}
	if name, ok := claims["name"]; ok && name != nil {
		return name.(string)
	}
	return claims["sub"].(string)
}

func ssoConfigured(set *settings.Settings) bool {
	return set.DexConfig != nil && len(set.DexConfig.Connectors) > 0
}

// getFreePort asks the kernel for a free open port that is ready to use.
func getFreePort() (int, error) {
	ln, err := net.Listen("tcp", "[::]:0")
	if err != nil {
		return 0, err
	}
	return ln.Addr().(*net.TCPAddr).Port, ln.Close()
}

// oauth2Login opens a browser, runs a temporary HTTP server to delegate OAuth2 login flow and
// returns the JWT token and a refresh token (if supported)
func oauth2Login(host string, plaintext bool) (string, string) {
	ctx := context.Background()
	port, err := getFreePort()
	errors.CheckError(err)
	var scheme = "https"
	if plaintext {
		scheme = "http"
	}
	conf := &oauth2.Config{
		ClientID: common.ArgoCDCLIClientAppID,
		Scopes:   []string{"openid", "profile", "email", "groups", "offline_access"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s://%s%s/auth", scheme, host, common.DexAPIEndpoint),
			TokenURL: fmt.Sprintf("%s://%s%s/token", scheme, host, common.DexAPIEndpoint),
		},
		RedirectURL: fmt.Sprintf("http://localhost:%d/auth/callback", port),
	}
	srv := &http.Server{Addr: ":" + strconv.Itoa(port)}
	var tokenString string
	var refreshToken string
	loginCompleted := make(chan struct{})

	callbackHandler := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			loginCompleted <- struct{}{}
		}()

		// Authorization redirect callback from OAuth2 auth flow.
		if errMsg := r.FormValue("error"); errMsg != "" {
			http.Error(w, errMsg+": "+r.FormValue("error_description"), http.StatusBadRequest)
			log.Fatal(errMsg)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			errMsg := fmt.Sprintf("no code in request: %q", r.Form)
			http.Error(w, errMsg, http.StatusBadRequest)
			log.Fatal(errMsg)
			return
		}
		tok, err := conf.Exchange(ctx, code)
		errors.CheckError(err)
		log.Info("Authentication successful")

		var ok bool
		tokenString, ok = tok.Extra("id_token").(string)
		if !ok {
			errMsg := "no id_token in token response"
			http.Error(w, errMsg, http.StatusInternalServerError)
			log.Fatal(errMsg)
			return
		}
		refreshToken, _ = tok.Extra("refresh_token").(string)
		log.Debugf("Token: %s", tokenString)
		log.Debugf("Refresh Token: %s", tokenString)
		successPage := `
		<div style="height:100px; width:100%!; display:flex; flex-direction: column; justify-content: center; align-items:center; background-color:#2ecc71; color:white; font-size:22"><div>Authentication successful!</div></div>
		<p style="margin-top:20px; font-size:18; text-align:center">Authentication was successful, you can now return to CLI. This page will close automatically</p>
		<script>window.onload=function(){setTimeout(this.close, 4000)}</script>
		`
		fmt.Fprintf(w, successPage)
	}
	http.HandleFunc("/auth/callback", callbackHandler)

	// add transport for self-signed certificate to context
	sslcli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, sslcli)

	// Redirect user to login & consent page to ask for permission for the scopes specified above.
	log.Info("Opening browser for authentication")
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	log.Infof("Authentication URL: %s", url)
	time.Sleep(1 * time.Second)
	err = open.Run(url)
	errors.CheckError(err)
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	<-loginCompleted
	_ = srv.Shutdown(ctx)
	return tokenString, refreshToken
}

func passwordLogin(acdClient argocdclient.Client, username, password string) string {
	username, password = cli.PromptCredentials(username, password)
	sessConn, sessionIf := acdClient.NewSessionClientOrDie()
	defer util.Close(sessConn)
	sessionRequest := session.SessionCreateRequest{
		Username: username,
		Password: password,
	}
	createdSession, err := sessionIf.Create(context.Background(), &sessionRequest)
	errors.CheckError(err)
	return createdSession.Token
}
