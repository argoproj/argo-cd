package commands

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	settingspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
	oidcutil "github.com/argoproj/argo-cd/v3/util/oidc"
	"github.com/argoproj/argo-cd/v3/util/rand"
	oidcconfig "github.com/argoproj/argo-cd/v3/util/settings"
)

// NewLoginCommand returns a new instance of `argocd login` command
func NewLoginCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		ctxName          string
		username         string
		password         string
		sso              bool
		callback         string
		ssoPort          int
		skipTestTLS      bool
		ssoLaunchBrowser bool
		noBrowser        bool
	)
	command := &cobra.Command{
		Use:   "login SERVER",
		Short: "Log in to Argo CD",
		Long: `Log in to Argo CD

A client certificate given with --client-crt and --client-crt-key is recorded in the context, so that
subsequent commands reuse it without having to specify the flags again. The certificate is referenced
by path and read on every invocation, which means certificates rotated by an external tool are picked
up automatically.`,
		Example: `# Login to Argo CD using a username and password
argocd login cd.argoproj.io

# Login to Argo CD using SSO
argocd login cd.argoproj.io --sso

# Login to Argo CD using SSO without a browser (device code flow)
argocd login cd.argoproj.io --sso --no-browser

# Login to an Argo CD instance behind a proxy requiring a client certificate
argocd login cd.argoproj.io --client-crt ~/certs/argocd.crt --client-crt-key ~/certs/argocd.key

# Configure direct access using Kubernetes API server
argocd login cd.argoproj.io --core`,
		Run: cli.WithSignalContext(func(c *cobra.Command, args []string, _ context.CancelFunc) {
			ctx := c.Context()

			var server string

			if len(args) != 1 && !clientOpts.PortForward && !clientOpts.Core {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			switch {
			case clientOpts.PortForward:
				server = "port-forward"
			case clientOpts.Core:
				server = "kubernetes"
			default:
				server = args[0]

				if !skipTestTLS {
					dialTime := 30 * time.Second
					tlsTestResult, err := grpc_util.TestTLS(server, dialTime)
					errors.CheckError(err)
					if !tlsTestResult.TLS {
						if !clientOpts.PlainText {
							if !cli.AskToProceed("WARNING: server is not configured with TLS. Proceed (y/n)? ") {
								os.Exit(1)
							}
							clientOpts.PlainText = true
						}
					} else if tlsTestResult.InsecureErr != nil {
						if !clientOpts.Insecure {
							if !cli.AskToProceed(fmt.Sprintf("WARNING: server certificate had error: %s. Proceed insecurely (y/n)? ", tlsTestResult.InsecureErr)) {
								os.Exit(1)
							}
							clientOpts.Insecure = true
						}
					}
				}
			}
			localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				localCfg = &localconfig.LocalConfig{}
			}

			serverCfg := localconfig.Server{
				Server:          server,
				PlainText:       clientOpts.PlainText,
				Insecure:        clientOpts.Insecure,
				GRPCWeb:         clientOpts.GRPCWeb,
				GRPCWebRootPath: clientOpts.GRPCWebRootPath,
				Core:            clientOpts.Core,
			}
			// An error simply means the server has not been logged into before, nothing to carry over.
			existingCfg, _ := localCfg.GetServer(server)
			errors.CheckError(applyCertConfig(&serverCfg, existingCfg, clientOpts.ClientCertFile, clientOpts.ClientCertKeyFile))

			loginOpts := argocdclient.ClientOptions{
				ConfigPath:           "",
				ServerAddr:           server,
				Insecure:             clientOpts.Insecure,
				PlainText:            clientOpts.PlainText,
				GRPCWeb:              clientOpts.GRPCWeb,
				GRPCWebRootPath:      clientOpts.GRPCWebRootPath,
				PortForward:          clientOpts.PortForward,
				PortForwardNamespace: clientOpts.PortForwardNamespace,
				Headers:              clientOpts.Headers,
				KubeOverrides:        clientOpts.KubeOverrides,
				ServerName:           clientOpts.ServerName,
			}
			loginOpts.ClientCertData, loginOpts.ClientCertKeyData, err = serverCfg.ClientCertPEM()
			errors.CheckError(err)

			if ctxName == "" {
				ctxName = server
				if loginOpts.GRPCWebRootPath != "" {
					rootPath := strings.TrimRight(strings.TrimLeft(loginOpts.GRPCWebRootPath, "/"), "/")
					ctxName = fmt.Sprintf("%s/%s", server, rootPath)
				}
			}

			// Perform the login
			var tokenString string
			var refreshToken string
			if !clientOpts.Core {
				if noBrowser && !sso {
					log.Fatal("--no-browser requires --sso")
				}
				acdClient := headless.NewClientOrDie(&loginOpts, c)
				setConn, setIf := acdClient.NewSettingsClientOrDieWithContext(ctx)
				defer utilio.Close(setConn)
				if !sso {
					tokenString = passwordLogin(ctx, acdClient, username, password)
				} else {
					httpClient, err := acdClient.HTTPClient()
					errors.CheckError(err)
					ctx = oidc.ClientContext(ctx, httpClient)
					acdSet, err := setIf.Get(ctx, &settingspkg.SettingsQuery{})
					errors.CheckError(err)
					oauth2conf, provider, err := acdClient.OIDCConfig(ctx, acdSet)
					errors.CheckError(err)
					if !noBrowser {
						tokenString, refreshToken = oauth2Login(ctx, callback, ssoPort, acdSet.GetOIDCConfig(), oauth2conf, provider, ssoLaunchBrowser, acdSet.GetDexConfig().GetDexAuthConnectorID())
					} else {
						if c.Flags().Changed("sso-port") {
							log.Warn("--sso-port is ignored when --no-browser is used")
						}
						if c.Flags().Changed("callback") {
							log.Warn("--callback is ignored when --no-browser is used")
						}
						tokenString, refreshToken = oauth2LoginNoBrowser(ctx, acdSet.GetOIDCConfig(), oauth2conf, httpClient)
					}
				}
				parser := jwt.NewParser(jwt.WithoutClaimsValidation())
				claims := jwt.MapClaims{}
				_, _, err := parser.ParseUnverified(tokenString, &claims)
				errors.CheckError(err)
				fmt.Printf("'%s' logged in successfully\n", userDisplayName(claims))
			}

			// login successful. Persist the config
			localCfg.UpsertServer(serverCfg)
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
			err = localconfig.WriteLocalConfig(*localCfg, clientOpts.ConfigPath)
			errors.CheckError(err)
			fmt.Printf("Context '%s' updated\n", ctxName)
		}),
	}
	command.Flags().StringVar(&ctxName, "name", "", "Name to use for the context")
	command.Flags().StringVar(&username, "username", "", "The username of an account to authenticate")
	command.Flags().StringVar(&password, "password", "", "The password of an account to authenticate")
	command.Flags().BoolVar(&sso, "sso", false, "Perform SSO login")
	command.Flags().BoolVar(&noBrowser, "no-browser", false, "Perform SSO login without a browser using the device code flow (requires --sso)")
	command.Flags().IntVar(&ssoPort, "sso-port", DefaultSSOLocalPort, "Port to run local OAuth2 login application")
	command.Flags().StringVar(&callback, "callback", "", "Scheme, Host and Port for the callback URL")
	command.Flags().BoolVar(&skipTestTLS, "skip-test-tls", false, "Skip testing whether the server is configured with TLS (this can help when the command hangs for no apparent reason)")
	command.Flags().BoolVar(&ssoLaunchBrowser, "sso-launch-browser", true, "Automatically launch the system default browser when performing SSO login")
	return command
}

// applyCertConfig fills in the certificate settings of the server context being logged into. The
// certificates of a previous login are carried over, so that --client-crt and --client-crt-key do
// not have to be repeated on every login, unless new ones are given on the command line.
func applyCertConfig(serverCfg *localconfig.Server, existing *localconfig.Server, clientCertFile, clientCertKeyFile string) error {
	if existing != nil {
		serverCfg.CACertificateAuthorityData = existing.CACertificateAuthorityData
		serverCfg.ClientCertificate = existing.ClientCertificate
		serverCfg.ClientCertificateKey = existing.ClientCertificateKey
		serverCfg.ClientCertificateData = existing.ClientCertificateData
		serverCfg.ClientCertificateKeyData = existing.ClientCertificateKeyData
	}
	switch {
	case clientCertFile != "" && clientCertKeyFile != "":
		// Persist absolute paths, since the CLI may later be invoked from a different directory.
		cert, err := filepath.Abs(clientCertFile)
		if err != nil {
			return err
		}
		key, err := filepath.Abs(clientCertKeyFile)
		if err != nil {
			return err
		}
		serverCfg.ClientCertificate = cert
		serverCfg.ClientCertificateKey = key
		serverCfg.ClientCertificateData = ""
		serverCfg.ClientCertificateKeyData = ""
	case clientCertFile != "" || clientCertKeyFile != "":
		return stderrors.New("--client-crt and --client-crt-key must always be specified together")
	}
	return nil
}

func userDisplayName(claims jwt.MapClaims) string {
	if email := jwtutil.StringField(claims, "email"); email != "" {
		return email
	}
	if name := jwtutil.StringField(claims, "name"); name != "" {
		return name
	}
	return jwtutil.GetUserIdentifier(claims)
}

// generatePKCECodeVerifierAndChallenge generates a PKCE code verifier and its
// corresponding S256 code challenge as defined in RFC 7636.
// The verifier is sent in the token request; the challenge is sent in the
// authorization / device-authorization request.
func generatePKCECodeVerifierAndChallenge() (verifier, challenge string, err error) {
	verifier, err = rand.StringFromCharset(
		43,
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~",
	)
	if err != nil {
		return "", "", err
	}
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

// oauth2Login opens a browser, runs a temporary HTTP server to delegate OAuth2 login flow and
// returns the JWT token and a refresh token (if supported)
func oauth2Login(
	ctx context.Context,
	callback string,
	port int,
	oidcSettings *settingspkg.OIDCConfig,
	oauth2conf *oauth2.Config,
	provider *oidc.Provider,
	ssoLaunchBrowser bool,
	dexAuthConnectorID string,
) (string, string) {
	redirectBase := callback
	if redirectBase == "" {
		redirectBase = "http://localhost:" + strconv.Itoa(port)
	}

	oauth2conf.RedirectURL = redirectBase + "/auth/callback"
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
	// According to the spec (https://www.rfc-editor.org/rfc/rfc6749#section-10.10), this must be guessable with
	// probability <= 2^(-128). The following call generates one of 52^24 random strings, ~= 2^136 possibilities.
	stateNonce, err := rand.String(24)
	errors.CheckError(err)
	var tokenString string
	var refreshToken string

	handleErr := func(w http.ResponseWriter, errMsg string) {
		http.Error(w, html.EscapeString(errMsg), http.StatusBadRequest)
		completionChan <- errMsg
	}

	// PKCE implementation of https://tools.ietf.org/html/rfc7636
	codeVerifier, codeChallenge, err := generatePKCECodeVerifierAndChallenge()
	errors.CheckError(err)

	// Authorization redirect callback from OAuth2 auth flow.
	// Handles both implicit and authorization code flow
	callbackHandler := func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("Callback: %s", r.URL)

		if formErr := r.FormValue("error"); formErr != "" {
			handleErr(w, fmt.Sprintf("%s: %s", formErr, r.FormValue("error_description")))
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
			fmt.Fprint(w, `<script>window.location.search = window.location.hash.substring(1)</script>`)
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
	var url string
	var oidcconfig oidcconfig.OIDCConfig
	grantType := oidcutil.InferGrantType(oidcConf)
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if claimsRequested := oidcSettings.GetIDTokenClaims(); claimsRequested != nil {
		opts = oidcutil.AppendClaimsAuthenticationRequestParameter(opts, claimsRequested)
	}
	// When bundled Dex is configured with a forced connector, redirect straight to it and
	// bypass Dex's connector selection screen (mirrors the browser login flow).
	if dexAuthConnectorID != "" {
		log.Debugf("force redirect to selected connector_id: %s", dexAuthConnectorID)
		opts = append(opts, oauth2.SetAuthURLParam("connector_id", dexAuthConnectorID))
	}

	switch grantType {
	case oidcutil.GrantTypeAuthorizationCode:
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge", codeChallenge))
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge_method", "S256"))
		if oidcconfig.DomainHint != "" {
			opts = append(opts, oauth2.SetAuthURLParam("domain_hint", oidcconfig.DomainHint))
		}
		url = oauth2conf.AuthCodeURL(stateNonce, opts...)
	case oidcutil.GrantTypeImplicit:
		url, err = oidcutil.ImplicitFlowURL(oauth2conf, stateNonce, opts...)
		errors.CheckError(err)
	default:
		log.Fatalf("Unsupported grant type: %v", grantType)
	}
	fmt.Printf("Performing %s flow login: %s\n", grantType, url)
	time.Sleep(1 * time.Second)
	ssoAuthFlow(url, ssoLaunchBrowser)
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
	fmt.Print("Authentication successful\n")
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Debugf("Token: %s", tokenString)
	log.Debugf("Refresh Token: %s", refreshToken)
	return tokenString, refreshToken
}

// buildVerificationPrompt returns the lines to print between the
// "authenticate" header and the "Waiting" footer.
//
// Priority:
//  1. verification_uri_complete — print as-is; the user just opens one URL.
//  2. Fallback per RFC 8628 §3.3 — show verification_uri and user_code
//     separately.
func buildVerificationPrompt(uriComplete, uri, userCode string) string {
	if uriComplete != "" {
		return "  " + uriComplete
	}
	return fmt.Sprintf("  %s\n\n  Enter the code: %s", uri, userCode)
}

// oauth2LoginNoBrowser implements the OAuth 2.0 Device Authorization Grant
// (RFC 8628) using golang.org/x/oauth2's built-in DeviceAuth / DeviceAccessToken
// methods. It prints a verification URL for the user to open in a browser and
// polls the token endpoint until authentication completes or is cancelled.
func oauth2LoginNoBrowser(
	ctx context.Context,
	oidcSettings *settingspkg.OIDCConfig,
	oauth2conf *oauth2.Config,
	httpClient *http.Client,
) (string, string) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Inject the custom Argo CD HTTP client (TLS settings, proxy, etc.) so
	// that golang.org/x/oauth2 uses it for all token endpoint requests.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	// Prefer the device authorization endpoint auto-discovered from the OIDC
	// discovery document (device_authorization_endpoint). Fall back to the
	// operator-configured DeviceURL for providers that do not advertise it.
	if oauth2conf.Endpoint.DeviceAuthURL == "" {
		oauth2conf.Endpoint.DeviceAuthURL = oidcSettings.GetDeviceURL()
	}
	if oauth2conf.Endpoint.TokenURL == "" {
		oauth2conf.Endpoint.TokenURL = oidcSettings.GetTokenURL()
	}
	if oauth2conf.Endpoint.DeviceAuthURL == "" || oauth2conf.Endpoint.TokenURL == "" {
		log.Fatal("OIDC provider does not advertise a device authorization or token endpoint; configure deviceURL/tokenURL in oidc.config")
	}

	// PKCE (RFC 7636): always include code_challenge in the device authorization
	// request. Providers that enforce PKCE globally (e.g. Keycloak with
	// "Require PKCE = S256") reject the device auth request without it.
	// Sending PKCE unconditionally is safe: compliant providers that do not
	// require it simply ignore the extra parameters.
	codeVerifier, codeChallenge, err := generatePKCECodeVerifierAndChallenge()
	errors.CheckError(err)

	// golang.org/x/oauth2's retrieveDeviceAuth creates its HTTP request via
	// http.NewRequest (not http.NewRequestWithContext), so context cancellation
	// (e.g. Ctrl+C) cannot abort the in-flight HTTP call. Work around this by
	// shallow-copying the HTTP client and setting a bounded Timeout; http.Client
	// always honours Timeout in Do() regardless of the request's own context.
	// 30 s is generous: a device-authorization endpoint must respond quickly.
	deviceAuthHTTPClient := *httpClient
	deviceAuthHTTPClient.Timeout = 30 * time.Second
	deviceAuthCtx := context.WithValue(ctx, oauth2.HTTPClient, &deviceAuthHTTPClient)

	da, err := oauth2conf.DeviceAuth(deviceAuthCtx,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	if err != nil {
		log.Fatalf("Failed to request device code: %s", err)
		return "", ""
	}

	// RFC 8628: prefer verification_uri_complete when present. Otherwise print
	// verification_uri and user_code separately so the user can enter the code
	// manually — which is the correct fallback per the spec.
	fmt.Printf("Open the following URL in your browser to authenticate:\n\n%s\n\nWaiting for authentication...\n",
		buildVerificationPrompt(da.VerificationURIComplete, da.VerificationURI, da.UserCode))

	tok, err := oauth2conf.DeviceAccessToken(ctx, da,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
	if err != nil {
		log.Fatalf("%s", err)
		return "", ""
	}

	idToken, _ := tok.Extra("id_token").(string)
	if idToken == "" {
		log.Fatal("OIDC provider did not return an id_token in the device flow token response")
		return "", ""
	}

	fmt.Print("Authentication successful\n")
	return idToken, tok.RefreshToken
}

func passwordLogin(ctx context.Context, acdClient argocdclient.Client, username, password string) string {
	username, password = cli.PromptCredentials(username, password)
	sessConn, sessionIf := acdClient.NewSessionClientOrDieWithContext(ctx)
	defer utilio.Close(sessConn)
	sessionRequest := sessionpkg.SessionCreateRequest{
		Username: username,
		Password: password,
	}
	createdSession, err := sessionIf.Create(ctx, &sessionRequest)
	errors.CheckError(err)
	return createdSession.Token
}

func ssoAuthFlow(url string, ssoLaunchBrowser bool) {
	if ssoLaunchBrowser {
		fmt.Print("Opening system default browser for authentication\n")
		err := open.Start(url)
		errors.CheckError(err)
	} else {
		fmt.Printf("To authenticate, copy-and-paste the following URL into your preferred browser: %s\n", url)
	}
}
