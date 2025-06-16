package commands

import (
	"context"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
func NewLoginCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		ctxName                string
		username               string
		password               string
		sso                    bool
		ssoPort                int
		ssoListenerIsSecure    bool
		ssoListenerHost        string
		ssoListenerCertFile    string
		ssoListenerCertKeyFile string
		skipTestTLS            bool
		ssoLaunchBrowser       bool
	)
	command := &cobra.Command{
		Use:   "login SERVER",
		Short: "Log in to Argo CD",
		Long:  "Log in to Argo CD",
		Example: `# Login to Argo CD using a username and password
argocd login cd.argoproj.io

# Login to Argo CD using SSO
argocd login cd.argoproj.io --sso

# Configure direct access using Kubernetes API server
argocd login cd.argoproj.io --core`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			var server string

			if len(args) != 1 && !globalClientOpts.PortForward && !globalClientOpts.Core {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			switch {
			case globalClientOpts.PortForward:
				server = "port-forward"
			case globalClientOpts.Core:
				server = "kubernetes"
			default:
				server = args[0]

				if !skipTestTLS {
					dialTime := 30 * time.Second
					tlsTestResult, err := grpc_util.TestTLS(server, dialTime)
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
			}
			clientOpts := argocdclient.ClientOptions{
				ConfigPath:           "",
				ServerAddr:           server,
				Insecure:             globalClientOpts.Insecure,
				PlainText:            globalClientOpts.PlainText,
				ClientCertFile:       globalClientOpts.ClientCertFile,
				ClientCertKeyFile:    globalClientOpts.ClientCertKeyFile,
				GRPCWeb:              globalClientOpts.GRPCWeb,
				GRPCWebRootPath:      globalClientOpts.GRPCWebRootPath,
				PortForward:          globalClientOpts.PortForward,
				PortForwardNamespace: globalClientOpts.PortForwardNamespace,
				Headers:              globalClientOpts.Headers,
				KubeOverrides:        globalClientOpts.KubeOverrides,
				ServerName:           globalClientOpts.ServerName,
			}

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
			if !globalClientOpts.Core {
				acdClient := headless.NewClientOrDie(&clientOpts, c)
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
					tokenString, refreshToken = oauth2Login(ctx, ssoPort, acdSet.GetOIDCConfig(), oauth2conf, provider, ssoLaunchBrowser, ssoListenerIsSecure, ssoListenerHost, ssoListenerCertFile, ssoListenerCertKeyFile)
				}
				parser := jwt.NewParser(jwt.WithoutClaimsValidation())
				claims := jwt.MapClaims{}
				_, _, err := parser.ParseUnverified(tokenString, &claims)
				errors.CheckError(err)
				fmt.Printf("'%s' logged in successfully\n", userDisplayName(claims))
			}

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
				Core:            globalClientOpts.Core,
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
	command.Flags().StringVar(&ctxName, "name", "", "Name to use for the context")
	command.Flags().StringVar(&username, "username", "", "The username of an account to authenticate")
	command.Flags().StringVar(&password, "password", "", "The password of an account to authenticate")
	command.Flags().BoolVar(&sso, "sso", false, "Perform SSO login")
	command.Flags().IntVar(&ssoPort, "sso-port", DefaultSSOLocalPort, "Port to run local OAuth2 login application")
	command.Flags().BoolVar(&skipTestTLS, "skip-test-tls", false, "Skip testing whether the server is configured with TLS (this can help when the command hangs for no apparent reason)")
	command.Flags().BoolVar(&ssoListenerIsSecure, "sso-use-tls", false, "Use TLS on local SSO callback listener")
	command.Flags().StringVar(&ssoListenerHost, "sso-listener-host", "localhost", "Host to use for local SSO callback")
	command.Flags().StringVar(&ssoListenerCertFile, "sso-listener-cert", "", "File containing the TLS x509 Certificate for SSO callback listener")
	command.Flags().StringVar(&ssoListenerCertKeyFile, "sso-listener-cert-key", "", "File containing the TLS private key for SSO callback listener")
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
	port int,
	oidcSettings *settingspkg.OIDCConfig,
	oauth2conf *oauth2.Config,
	provider *oidc.Provider,
	ssoLaunchBrowser bool,
	ssoListenerIsSecure bool,
	ssoListenerHost string,
	ssoListenerCertFile string,
	ssoListenerCertKeyFile string,
) (string, string) {
	var callbackProtocol string
	if ssoListenerIsSecure {
		callbackProtocol = "https"
	} else {
		callbackProtocol = "http"
	}
	oauth2conf.RedirectURL = fmt.Sprintf("%s://%s:%d/auth/callback", callbackProtocol, ssoListenerHost, port)
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
	srv := &http.Server{Addr: fmt.Sprintf("%s:", ssoListenerHost) + strconv.Itoa(port)}
	http.HandleFunc("/auth/callback", callbackHandler)

	// Redirect user to login & consent page to ask for permission for the scopes specified above.
	var url string
	var oidcconfig oidcconfig.OIDCConfig
	grantType := oidcutil.InferGrantType(oidcConf)
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if claimsRequested := oidcSettings.GetIDTokenClaims(); claimsRequested != nil {
		opts = oidcutil.AppendClaimsAuthenticationRequestParameter(opts, claimsRequested)
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
		if !ssoListenerIsSecure {
			log.Println("Starting HTTP server (TLS disabled)")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Temporary HTTP server failed: %s", err)
			}
		} else {
			// TLS is enabled
			var finalCertFile, finalKeyFile string
			var generatedSelfSigned bool = false

			if ssoListenerCertFile != "" && ssoListenerCertKeyFile != "" {
				// Use provided certificate and key files
				// Ensure they are absolute paths or resolve them relative to current dir
				var err error
				finalCertFile, err = filepath.Abs(ssoListenerCertFile)
				if err != nil {
					log.Fatalf("Error resolving cert file path %s: %v", ssoListenerCertFile, err)
				}
				finalKeyFile, err = filepath.Abs(ssoListenerCertKeyFile)
				if err != nil {
					log.Fatalf("Error resolving key file path %s: %v", ssoListenerCertKeyFile, err)
				}

				if _, err := os.Stat(finalCertFile); os.IsNotExist(err) {
					log.Fatalf("Certificate file not found: %s", finalCertFile)
				}
				if _, err := os.Stat(finalKeyFile); os.IsNotExist(err) {
					log.Fatalf("Key file not found: %s", finalKeyFile)
				}
				log.Printf("Starting HTTPS server using provided certificate: %s and key: %s", finalCertFile, finalKeyFile)
			} else {
				// Generate self-signed certificate for listener host
				log.Printf("ssoListenerCertFile or ssoListenerKeyFile not specified, generating self-signed certificate for %s...\n", ssoListenerHost)
				var err error
				finalCertFile, finalKeyFile, err = generateSelfSignedCert(ssoListenerHost)
				if err != nil {
					log.Fatalf("Could not generate self-signed certificate: %v", err)
				}
				generatedSelfSigned = true
				log.Printf("Starting HTTPS server using self-signed certificate (cert: %s, key: %s)", finalCertFile, finalKeyFile)

				// Clean up temporary self-signed certs on exit
				defer func() {
					if generatedSelfSigned {
						log.Printf("Cleaning up temporary self-signed certificate files: %s, %s", finalCertFile, finalKeyFile)
						os.Remove(finalCertFile)
						os.Remove(finalKeyFile)
					}
				}()
			}

			// Optional: Configure TLS settings (e.g., minimum TLS version)
			srv.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}

			if err := srv.ListenAndServeTLS(finalCertFile, finalKeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Temporary HTTPS server failed: %s", err)
			}
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
		fmt.Printf("Opening system default browser for authentication\n")
		err := open.Start(url)
		errors.CheckError(err)
	} else {
		fmt.Printf("To authenticate, copy-and-paste the following URL into your preferred browser: %s\n", url)
	}
}

// generateSelfSignedCert creates a self-signed X.509 certificate for the given host.
// It returns the paths to the generated certificate and key files, or an error.
func generateSelfSignedCert(host string) (string, string, error) {
	priv, err := rsa.GenerateKey(cryptoRand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 1 year validity

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := cryptoRand.Int(cryptoRand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"argo-cd cli sso callback handler"},
			CommonName:   host,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host}, // Important for browser validation
	}

	derBytes, err := x509.CreateCertificate(cryptoRand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Create temporary files for cert and key
	certFile, err := os.CreateTemp("", "selfsigned-cert-*.pem")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp cert file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", "", fmt.Errorf("failed to write cert data to %s: %w", certFile.Name(), err)
	}

	keyFile, err := os.CreateTemp("", "selfsigned-key-*.pem")
	if err != nil {
		os.Remove(certFile.Name()) // Clean up cert file if key file creation fails
		return "", "", fmt.Errorf("failed to create temp key file: %w", err)
	}
	defer keyFile.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		os.Remove(certFile.Name())
		os.Remove(keyFile.Name())
		return "", "", fmt.Errorf("failed to marshal private key: %w", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		os.Remove(certFile.Name())
		os.Remove(keyFile.Name())
		return "", "", fmt.Errorf("failed to write key data to %s: %w", keyFile.Name(), err)
	}

	return certFile.Name(), keyFile.Name(), nil
}
