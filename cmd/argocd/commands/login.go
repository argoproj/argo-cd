package commands

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/dgrijalva/jwt-go/v4"
	log "github.com/sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	sessionpkg "github.com/argoproj/argo-cd/pkg/apiclient/session"
	settingspkg "github.com/argoproj/argo-cd/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/errors"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/io"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	"github.com/argoproj/argo-cd/util/localconfig"
	oidcutil "github.com/argoproj/argo-cd/util/oidc"
	"github.com/argoproj/argo-cd/util/rand"
)

// NewLoginCommand returns a new instance of `argocd login` command
func NewLoginCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		ctxName  string
		username string
		password string
		sso      bool
		ssoPort  int
	)
	var command = &cobra.Command{
		Use:   "login SERVER",
		Short: "Log in to Argo CD",
		Long:  "Log in to Argo CD",
		Run: func(c *cobra.Command, args []string) {
			var server string

			if len(args) != 1 && !globalClientOpts.PortForward {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			if globalClientOpts.PortForward {
				server = "port-forward"
			} else {
				server = args[0]
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
			}
			clientOpts := argocdclient.ClientOptions{
				ConfigPath:           "",
				ServerAddr:           server,
				Insecure:             globalClientOpts.Insecure,
				PlainText:            globalClientOpts.PlainText,
				GRPCWeb:              globalClientOpts.GRPCWeb,
				GRPCWebRootPath:      globalClientOpts.GRPCWebRootPath,
				PortForward:          globalClientOpts.PortForward,
				PortForwardNamespace: globalClientOpts.PortForwardNamespace,
				Headers:              globalClientOpts.Headers,
			}
			acdClient := argocdclient.NewClientOrDie(&clientOpts)
			setConn, setIf := acdClient.NewSettingsClientOrDie()
			defer io.Close(setConn)

			if ctxName == "" {
				ctxName = server
				if globalClientOpts.GRPCWebRootPath != "" {
					rootPath := strings.TrimRight(strings.TrimLeft(globalClientOpts.GRPCWebRootPath, "/"), "/")
					ctxName = fmt.Sprintf("%s/%s", server, rootPath)
				}
			}

			// Perform the login
			var tokenString string
			var refreshToken string
			if !sso {
				tokenString = passwordLogin(acdClient, username, password)
			} else {
				ctx := context.Background()
				httpClient, err := acdClient.HTTPClient()
				errors.CheckError(err)
				ctx = oidc.ClientContext(ctx, httpClient)
				acdSet, err := setIf.Get(ctx, &settingspkg.SettingsQuery{})
				errors.CheckError(err)
				oauth2conf, provider, err := acdClient.OIDCConfig(ctx, acdSet)
				errors.CheckError(err)
				tokenString, refreshToken = oauth2Login(ctx, ssoPort, acdSet.GetOIDCConfig(), oauth2conf, provider)
			}

			parser := &jwt.Parser{
				ValidationHelper: jwt.NewValidationHelper(jwt.WithoutClaimsValidation()),
			}
			claims := jwt.MapClaims{}
			_, _, err := parser.ParseUnverified(tokenString, &claims)
			errors.CheckError(err)

			fmt.Printf("'%s' logged in successfully\n", userDisplayName(claims))
			// login successful. Persist the config
			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				localCfg = &localconfig.LocalConfig{}
			}
			localCfg.UpsertServer(localconfig.Server{
				Server:          server,
				PlainText:       globalClientOpts.PlainText,
				Insecure:        globalClientOpts.Insecure,
				GRPCWeb:         globalClientOpts.GRPCWeb,
				GRPCWebRootPath: globalClientOpts.GRPCWebRootPath,
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
	command.Flags().BoolVar(&sso, "sso", false, "perform SSO login")
	command.Flags().IntVar(&ssoPort, "sso-port", DefaultSSOLocalPort, "port to run local OAuth2 login application")
	return command
}

func userDisplayName(claims jwt.MapClaims) string {
	if email := jwtutil.StringField(claims, "email"); email != "" {
		return email
	}
	if name := jwtutil.StringField(claims, "name"); name != "" {
		return name
	}
	return jwtutil.StringField(claims, "sub")
}

// oauth2Login opens a browser, runs a temporary HTTP server to delegate OAuth2 login flow and
// returns the JWT token and a refresh token (if supported)
func oauth2Login(ctx context.Context, port int, oidcSettings *settingspkg.OIDCConfig, oauth2conf *oauth2.Config, provider *oidc.Provider) (string, string) {
	oauth2conf.RedirectURL = fmt.Sprintf("http://localhost:%d/auth/callback", port)
	oidcConf, err := oidcutil.ParseConfig(provider)
	errors.CheckError(err)
	log.Debug("OIDC Configuration:")
	log.Debugf("  supported_scopes: %v", oidcConf.ScopesSupported)
	log.Debugf("  response_types_supported: %v", oidcConf.ResponseTypesSupported)

	// handledRequests ensures we do not handle more requests than necessary
	handledRequests := 0
	// completionChan is to signal flow completed. Non-empty string indicates error
	completionChan := make(chan string)
	// stateNonce is an OAuth2 state nonce
	stateNonce := rand.RandString(10)
	var tokenString string
	var refreshToken string

	handleErr := func(w http.ResponseWriter, errMsg string) {
		http.Error(w, errMsg, http.StatusBadRequest)
		completionChan <- errMsg
	}

	// PKCE implementation of https://tools.ietf.org/html/rfc7636
	codeVerifier := rand.RandStringCharset(43, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
	codeChallengeHash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(codeChallengeHash[:])

	// Authorization redirect callback from OAuth2 auth flow.
	// Handles both implicit and authorization code flow
	callbackHandler := func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("Callback: %s", r.URL)

		if formErr := r.FormValue("error"); formErr != "" {
			handleErr(w, formErr+": "+r.FormValue("error_description"))
			return
		}

		handledRequests++
		if handledRequests > 2 {
			// Since implicit flow will redirect back to ourselves, this counter ensures we do not
			// fallinto a redirect loop (e.g. user visits the page by hand)
			handleErr(w, "Unable to complete login flow: too many redirects")
			return
		}

		if len(r.Form) == 0 {
			// If we get here, no form data was set. We presume to be performing an implicit login
			// flow where the id_token is contained in a URL fragment, making it inaccessible to be
			// read from the request. This javascript will redirect the browser to send the
			// fragments as query parameters so our callback handler can read and return token.
			fmt.Fprintf(w, `<script>window.location.search = window.location.hash.substring(1)</script>`)
			return
		}

		if state := r.FormValue("state"); state != stateNonce {
			handleErr(w, "Unknown state nonce")
			return
		}

		tokenString = r.FormValue("id_token")
		if tokenString == "" {
			code := r.FormValue("code")
			if code == "" {
				handleErr(w, fmt.Sprintf("no code in request: %q", r.Form))
				return
			}
			opts := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("code_verifier", codeVerifier)}
			tok, err := oauth2conf.Exchange(ctx, code, opts...)
			if err != nil {
				handleErr(w, err.Error())
				return
			}
			var ok bool
			tokenString, ok = tok.Extra("id_token").(string)
			if !ok {
				handleErr(w, "no id_token in token response")
				return
			}
			refreshToken, _ = tok.Extra("refresh_token").(string)
		}
		successPage := `
		<div style="height:100px; width:100%!; display:flex; flex-direction: column; justify-content: center; align-items:center; background-color:#2ecc71; color:white; font-size:22"><div>Authentication successful!</div></div>
		<p style="margin-top:20px; font-size:18; text-align:center">Authentication was successful, you can now return to CLI. This page will close automatically</p>
		<script>window.onload=function(){setTimeout(this.close, 4000)}</script>
		`
		fmt.Fprint(w, successPage)
		completionChan <- ""
	}
	srv := &http.Server{Addr: "localhost:" + strconv.Itoa(port)}
	http.HandleFunc("/auth/callback", callbackHandler)

	// Redirect user to login & consent page to ask for permission for the scopes specified above.
	fmt.Printf("Opening browser for authentication\n")

	var url string
	grantType := oidcutil.InferGrantType(oidcConf)
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if claimsRequested := oidcSettings.GetIDTokenClaims(); claimsRequested != nil {
		opts = oidcutil.AppendClaimsAuthenticationRequestParameter(opts, claimsRequested)
	}

	switch grantType {
	case oidcutil.GrantTypeAuthorizationCode:
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge", codeChallenge))
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge_method", "S256"))
		url = oauth2conf.AuthCodeURL(stateNonce, opts...)
	case oidcutil.GrantTypeImplicit:
		url = oidcutil.ImplicitFlowURL(oauth2conf, stateNonce, opts...)
	default:
		log.Fatalf("Unsupported grant type: %v", grantType)
	}
	fmt.Printf("Performing %s flow login: %s\n", grantType, url)
	time.Sleep(1 * time.Second)
	err = open.Start(url)
	errors.CheckError(err)
	go func() {
		log.Debugf("Listen: %s", srv.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Temporary HTTP server failed: %s", err)
		}
	}()
	errMsg := <-completionChan
	if errMsg != "" {
		log.Fatal(errMsg)
	}
	fmt.Printf("Authentication successful\n")
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Debugf("Token: %s", tokenString)
	log.Debugf("Refresh Token: %s", refreshToken)
	return tokenString, refreshToken
}

func passwordLogin(acdClient argocdclient.Client, username, password string) string {
	username, password = cli.PromptCredentials(username, password)
	sessConn, sessionIf := acdClient.NewSessionClientOrDie()
	defer io.Close(sessConn)
	sessionRequest := sessionpkg.SessionCreateRequest{
		Username: username,
		Password: password,
	}
	createdSession, err := sessionIf.Create(context.Background(), &sessionRequest)
	errors.CheckError(err)
	return createdSession.Token
}
