package commands

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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
		browserless      bool
	)
	command := &cobra.Command{
		Use:   "login SERVER",
		Short: "Log in to Argo CD",
		Long:  "Log in to Argo CD",
		Example: `# Login to Argo CD using a username and password
argocd login cd.argoproj.io

# Login to Argo CD using SSO
argocd login cd.argoproj.io --sso

# Login to Argo CD using SSO without a browser (device code flow)
argocd login cd.argoproj.io --sso --browserless

# Configure direct access using Kubernetes API server
argocd login cd.argoproj.io --core`,
		Run: func(c *cobra.Command, args []string) {
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
			loginOpts := argocdclient.ClientOptions{
				ConfigPath:           "",
				ServerAddr:           server,
				Insecure:             clientOpts.Insecure,
				PlainText:            clientOpts.PlainText,
				ClientCertFile:       clientOpts.ClientCertFile,
				ClientCertKeyFile:    clientOpts.ClientCertKeyFile,
				GRPCWeb:              clientOpts.GRPCWeb,
				GRPCWebRootPath:      clientOpts.GRPCWebRootPath,
				PortForward:          clientOpts.PortForward,
				PortForwardNamespace: clientOpts.PortForwardNamespace,
				Headers:              clientOpts.Headers,
				KubeOverrides:        clientOpts.KubeOverrides,
				ServerName:           clientOpts.ServerName,
			}

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
				acdClient := headless.NewClientOrDie(&loginOpts, c)
				setConn, setIf := acdClient.NewSettingsClientOrDie()
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
					if !browserless {
						tokenString, refreshToken = oauth2Login(ctx, callback, ssoPort, acdSet.GetOIDCConfig(), oauth2conf, provider, ssoLaunchBrowser, acdSet.GetDexConfig().GetDexAuthConnectorID())
					} else {
						tokenString, refreshToken = oauth2LoginBrowserless(ctx, acdSet.GetOIDCConfig(), oauth2conf, httpClient)
					}
				}
				parser := jwt.NewParser(jwt.WithoutClaimsValidation())
				claims := jwt.MapClaims{}
				_, _, err := parser.ParseUnverified(tokenString, &claims)
				errors.CheckError(err)
				fmt.Printf("'%s' logged in successfully\n", userDisplayName(claims))
			}

			// login successful. Persist the config
			localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				localCfg = &localconfig.LocalConfig{}
			}
			localCfg.UpsertServer(localconfig.Server{
				Server:          server,
				PlainText:       clientOpts.PlainText,
				Insecure:        clientOpts.Insecure,
				GRPCWeb:         clientOpts.GRPCWeb,
				GRPCWebRootPath: clientOpts.GRPCWebRootPath,
				Core:            clientOpts.Core,
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
			err = localconfig.WriteLocalConfig(*localCfg, clientOpts.ConfigPath)
			errors.CheckError(err)
			fmt.Printf("Context '%s' updated\n", ctxName)
		},
	}
	command.Flags().StringVar(&ctxName, "name", "", "Name to use for the context")
	command.Flags().StringVar(&username, "username", "", "The username of an account to authenticate")
	command.Flags().StringVar(&password, "password", "", "The password of an account to authenticate")
	command.Flags().BoolVar(&sso, "sso", false, "Perform SSO login")
	command.Flags().BoolVar(&browserless, "browserless", false, "Perform SSO login without a browser")
	command.Flags().IntVar(&ssoPort, "sso-port", DefaultSSOLocalPort, "Port to run local OAuth2 login application")
	command.Flags().StringVar(&callback, "callback", "", "Scheme, Host and Port for the callback URL")
	command.Flags().BoolVar(&skipTestTLS, "skip-test-tls", false, "Skip testing whether the server is configured with TLS (this can help when the command hangs for no apparent reason)")
	command.Flags().BoolVar(&ssoLaunchBrowser, "sso-launch-browser", true, "Automatically launch the system default browser when performing SSO login")
	return command
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
	codeVerifier, err := rand.StringFromCharset(
		43,
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~",
	)
	errors.CheckError(err)
	codeChallengeHash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(codeChallengeHash[:])

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

// httpDoer is a minimal interface over *http.Client, allowing unit tests to
// inject a fake transport without spinning up a real network connection.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// requestDeviceCode performs Step 1 of RFC 8628: POST to deviceURL and return
// the device authorization response.
func requestDeviceCode(ctx context.Context, client httpDoer, deviceURL, clientID, scope string) (*oidcutil.OIDCDeviceCodeResponseBody, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scope)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed: %s — %s", resp.Status, string(body))
	}

	var result oidcutil.OIDCDeviceCodeResponseBody
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}
	return &result, nil
}

// pollForToken performs Step 3 of RFC 8628: poll tokenURL until the device
// code is authorized, the deadline is reached, or the context is cancelled.
// pollInterval is the time to wait between attempts; deadline is the absolute
// expiry time of the device code.
func pollForToken(ctx context.Context, client httpDoer, tokenURL, clientID, deviceCode string, pollInterval time.Duration, deadline time.Time) (string, string, error) {
	tokenData := url.Values{}
	tokenData.Set("client_id", clientID)
	tokenData.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	tokenData.Set("device_code", deviceCode)

	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	for {
		if time.Now().After(deadline) {
			return "", "", stderrors.New("device code expired before authentication completed")
		}
		select {
		case <-ctx.Done():
			return "", "", stderrors.New("authentication cancelled")
		case <-timer.C:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(tokenData.Encode()))
		if err != nil {
			return "", "", fmt.Errorf("build token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			return "", "", fmt.Errorf("poll token endpoint: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil {
				switch errResp.Error {
				case "authorization_pending":
					timer.Reset(pollInterval)
					continue
				case "slow_down":
					pollInterval += 5 * time.Second
					timer.Reset(pollInterval)
					continue
				case "expired_token":
					return "", "", stderrors.New("device code expired before authentication completed")
				case "access_denied":
					return "", "", stderrors.New("access denied during device authorization")
				}
			}
			return "", "", fmt.Errorf("token request failed: %s — %s", resp.Status, string(body))
		}

		var tokenMap map[string]any
		if err := json.Unmarshal(body, &tokenMap); err != nil {
			return "", "", fmt.Errorf("decode token response: %w", err)
		}
		idToken, _ := tokenMap["id_token"].(string)
		if idToken == "" {
			return "", "", stderrors.New("no id_token in token response")
		}
		refreshToken, _ := tokenMap["refresh_token"].(string)
		return idToken, refreshToken, nil
	}
}

// buildVerificationPrompt returns the lines to print between the
// "authenticate" header and the "Waiting" footer.
//
// Priority:
//  1. verification_uri_complete — print as-is; the user just opens one URL.
//  2. verification_uri parseable — append user_code as a query parameter
//     (properly encoded) and also show the base URI + code for manual entry.
//  3. Fallback — print the URI and code as plain text.
func buildVerificationPrompt(uriComplete, uri, userCode string) string {
	if uriComplete != "" {
		return "  " + uriComplete
	}
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Sprintf("  %s\n\n  Enter the code: %s", uri, userCode)
	}
	q := u.Query()
	q.Set("user_code", userCode)
	u.RawQuery = q.Encode()
	return fmt.Sprintf("  %s\n\n  Or visit %s and enter the code: %s", u.String(), uri, userCode)
}

// oauth2LoginBrowserless implements the OAuth 2.0 Device Authorization Grant
// (RFC 8628). It prints a verification URL for the user to open in a browser
// and polls the token endpoint until authentication completes or is cancelled.
func oauth2LoginBrowserless(
	ctx context.Context,
	oidcSettings *settingspkg.OIDCConfig,
	oauth2conf *oauth2.Config,
	httpClient *http.Client,
) (string, string) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Prefer the device authorization endpoint auto-discovered from the OIDC
	// discovery document (device_authorization_endpoint). Fall back to the
	// operator-configured DeviceURL for providers that do not advertise it.
	deviceURL := oauth2conf.Endpoint.DeviceAuthURL
	if deviceURL == "" {
		deviceURL = oidcSettings.GetDeviceURL()
	}
	tokenURL := oauth2conf.Endpoint.TokenURL
	if tokenURL == "" {
		tokenURL = oidcSettings.GetTokenURL()
	}

	deviceResp, err := requestDeviceCode(ctx, httpClient, deviceURL, oauth2conf.ClientID, strings.Join(oauth2conf.Scopes, " "))
	if err != nil {
		log.Fatalf("Failed to request device code: %s", err)
		return "", ""
	}

	// RFC 8628: prefer verification_uri_complete when present. Otherwise print
	// verification_uri and user_code separately so the user can enter the code
	// manually — which is the correct fallback per the spec. If the provider
	// returns a verification_uri with query parameters we still offer a
	// synthesized URL as a convenience, with user_code properly encoded.
	fmt.Printf("Open the following URL in your browser to authenticate:\n\n%s\n\nWaiting for authentication...\n",
		buildVerificationPrompt(deviceResp.VerificationUriComplete, deviceResp.VerificationUri, deviceResp.UserCode))

	interval := deviceResp.Interval
	if interval <= 0 {
		interval = 5
	}
	expiresIn := deviceResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 300
	}
	idToken, refreshToken, err := pollForToken(ctx, httpClient, tokenURL, oauth2conf.ClientID, deviceResp.DeviceCode,
		time.Duration(interval)*time.Second,
		time.Now().Add(time.Duration(expiresIn)*time.Second),
	)
	if err != nil {
		log.Fatalf("%s", err)
		return "", ""
	}

	fmt.Print("Authentication successful\n")
	return idToken, refreshToken
}

func passwordLogin(ctx context.Context, acdClient argocdclient.Client, username, password string) string {
	username, password = cli.PromptCredentials(username, password)
	sessConn, sessionIf := acdClient.NewSessionClientOrDie()
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
