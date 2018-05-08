package dex

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/coreos/dex/api"
	oidc "github.com/coreos/go-oidc"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

const (
	// DexReverseProxyAddr is the address of the Dex OIDC server, which we run a reverse proxy against
	DexReverseProxyAddr = "http://localhost:5556"
	// DexgRPCAPIAddr is the address to the Dex gRPC API server for managing dex. This is assumed to run
	// locally (as a sidecar)
	DexgRPCAPIAddr = "localhost:5557"
	// DexAPIEndpoint is the endpoint where we serve the Dex API server
	DexAPIEndpoint = "/api/dex"
	// LoginEndpoint is ArgoCD's shorthand login endpoint which redirects to dex's OAuth 2.0 provider's consent page
	LoginEndpoint = "/auth/login"
	// CallbackEndpoint is ArgoCD's final callback endpoint we reach after OAuth 2.0 login flow has been completed
	CallbackEndpoint = "/auth/callback"
	// envVarSSODebug is an environment variable to enable additional OAuth debugging in the API server
	envVarSSODebug = "ARGOCD_SSO_DEBUG"
)

type DexAPIClient struct {
	api.DexClient
}

// NewDexHTTPReverseProxy returns a reverse proxy to the DEX server. Dex is assumed to be configured
// with the external issuer URL muxed to the same path configured in server.go. In other words, if
// ArgoCD API server wants to proxy requests at /api/dex, then the dex config yaml issuer URL should
// also be /api/dex (e.g. issuer: https://argocd.example.com/api/dex)
func NewDexHTTPReverseProxy() func(writer http.ResponseWriter, request *http.Request) {
	target, err := url.Parse(DexReverseProxyAddr)
	errors.CheckError(err)
	proxy := httputil.NewSingleHostReverseProxy(target)
	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}

func NewDexClient() (*DexAPIClient, error) {
	conn, err := grpc.Dial(DexgRPCAPIAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %v", DexgRPCAPIAddr, err)
	}
	apiClient := DexAPIClient{
		api.NewDexClient(conn),
	}
	return &apiClient, nil
}

// WaitUntilReady waits until the dex gRPC server is responding
func (d *DexAPIClient) WaitUntilReady() {
	log.Info("Waiting for dex to become ready")
	ctx := context.Background()
	for {
		vers, err := d.GetVersion(ctx, &api.VersionReq{})
		if err == nil {
			log.Infof("Dex %s (API: %d) up and running", vers.Server, vers.Api)
			return
		}
		time.Sleep(1 * time.Second)
	}
}

// TODO: implement proper state management
const exampleAppState = "I wish to wash my irish wristwatch"

type ClientApp struct {
	// OAuth2 client ID of this application (e.g. argo-cd)
	clientID string
	// OAuth2 client secret of this application
	clientSecret string
	// Callback URL for OAuth2 responses (e.g. https://argocd.example.com/auth/callback)
	redirectURI string
	// URL of the issuer (e.g. https://argocd.example.com/api/dex)
	issuerURL string

	Path string

	verifier *oidc.IDTokenVerifier
	provider *oidc.Provider

	// Does the provider use "offline_access" scope to request a refresh token
	// or does it use "access_type=offline" (e.g. Google)?
	offlineAsScope bool

	client *http.Client

	// sessionMgr creates and validates sessions
	sessionMgr session.SessionManager

	// secureCookie indicates if the cookie should be set with the Secure flag, meaning it should
	// only ever be sent over HTTPS. This value is inferred by the scheme of the redirectURI.
	secureCookie bool
}

// NewClientApp will register the ArgoCD client app in Dex and return an object which has HTTP
// handlers for handling the HTTP responses for login and callback
func NewClientApp(clientBaseURL string, serverSecretKey []byte, tlsConfig *tls.Config) (*ClientApp, error) {
	redirectURI := clientBaseURL + CallbackEndpoint
	issuerURL := clientBaseURL + DexAPIEndpoint
	log.Infof("Creating client app (redirectURI: %s, issuerURL: %s)", redirectURI, issuerURL)
	a := ClientApp{
		clientID:     ArgoCDClientAppID,
		clientSecret: formulateOAuthClientSecret(serverSecretKey),
		redirectURI:  redirectURI,
		issuerURL:    issuerURL,
	}
	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, fmt.Errorf("parse redirect-uri: %v", err)
	}
	a.Path = u.Path
	a.secureCookie = bool(u.Scheme == "https")

	a.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	if os.Getenv(envVarSSODebug) == "1" {
		a.client.Transport = debugTransport{a.client.Transport}
	}
	a.sessionMgr = session.MakeSessionManager(serverSecretKey)
	return &a, nil
}

type debugTransport struct {
	t http.RoundTripper
}

func (d debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, err
	}
	log.Printf("%s", reqDump)

	resp, err := d.t.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	log.Printf("%s", respDump)
	return resp, nil
}

// Initialize initializes the client app. The OIDC provider must be running
func (a *ClientApp) Initialize() error {
	log.Info("Initializing client app")
	ctx := oidc.ClientContext(context.Background(), a.client)
	provider, err := oidc.NewProvider(ctx, a.issuerURL)
	if err != nil {
		return fmt.Errorf("Failed to query provider %q: %v", a.issuerURL, err)
	}
	var s struct {
		// What scopes does a provider support?
		// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
		ScopesSupported []string `json:"scopes_supported"`
	}
	if err := provider.Claims(&s); err != nil {
		return fmt.Errorf("Failed to parse provider scopes_supported: %v", err)
	}
	log.Infof("OpenID supported scopes: %v", s.ScopesSupported)

	a.provider = provider
	a.verifier = provider.Verifier(&oidc.Config{ClientID: a.clientID})
	if len(s.ScopesSupported) == 0 {
		// scopes_supported is a "RECOMMENDED" discovery claim, not a required
		// one. If missing, assume that the provider follows the spec and has
		// an "offline_access" scope.
		a.offlineAsScope = true
	} else {
		// See if scopes_supported has the "offline_access" scope.
		a.offlineAsScope = func() bool {
			for _, scope := range s.ScopesSupported {
				if scope == oidc.ScopeOfflineAccess {
					return true
				}
			}
			return false
		}()
	}
	return nil
}

func (a *ClientApp) oauth2Config(scopes []string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint:     a.provider.Endpoint(),
		Scopes:       scopes,
		RedirectURL:  a.redirectURI,
	}
}

func (a *ClientApp) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var authCodeURL string
	scopes := []string{"openid", "profile", "email", "groups"}
	if r.FormValue("offline_access") != "yes" {
		authCodeURL = a.oauth2Config(scopes).AuthCodeURL(exampleAppState)
	} else if a.offlineAsScope {
		scopes = append(scopes, "offline_access")
		authCodeURL = a.oauth2Config(scopes).AuthCodeURL(exampleAppState)
	} else {
		authCodeURL = a.oauth2Config(scopes).AuthCodeURL(exampleAppState, oauth2.AccessTypeOffline)
	}
	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

func (a *ClientApp) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		token *oauth2.Token
	)

	ctx := oidc.ClientContext(r.Context(), a.client)
	oauth2Config := a.oauth2Config(nil)
	switch r.Method {
	case "GET":
		// Authorization redirect callback from OAuth2 auth flow.
		if errMsg := r.FormValue("error"); errMsg != "" {
			http.Error(w, errMsg+": "+r.FormValue("error_description"), http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			http.Error(w, fmt.Sprintf("no code in request: %q", r.Form), http.StatusBadRequest)
			return
		}
		if state := r.FormValue("state"); state != exampleAppState {
			http.Error(w, fmt.Sprintf("expected state %q got %q", exampleAppState, state), http.StatusBadRequest)
			return
		}
		token, err = oauth2Config.Exchange(ctx, code)
	case "POST":
		// Form request from frontend to refresh a token.
		refresh := r.FormValue("refresh_token")
		if refresh == "" {
			http.Error(w, fmt.Sprintf("no refresh_token in request: %q", r.Form), http.StatusBadRequest)
			return
		}
		t := &oauth2.Token{
			RefreshToken: refresh,
			Expiry:       time.Now().Add(-time.Hour),
		}
		token, err = oauth2Config.TokenSource(ctx, t).Token()
	default:
		http.Error(w, fmt.Sprintf("method not implemented: %s", r.Method), http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in token response", http.StatusInternalServerError)
		return
	}

	idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to verify ID token: %v", err), http.StatusInternalServerError)
		return
	}
	var claims jwt.MapClaims
	err = idToken.Claims(&claims)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to unmarshal claims: %v", err), http.StatusInternalServerError)
		return
	}
	clientToken, err := a.sessionMgr.SignClaims(claims)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to sign identity provider claims: %v", err), http.StatusInternalServerError)
		return
	}
	flags := []string{"path=/"}
	if a.secureCookie {
		flags = append(flags, "Secure")
	}
	cookie := session.MakeCookieMetadata(common.AuthCookieName, clientToken, flags...)
	w.Header().Set("Set-Cookie", cookie)
	if os.Getenv(envVarSSODebug) == "1" {
		claimsJSON, _ := json.MarshalIndent(claims, "", "  ")
		renderToken(w, a.redirectURI, rawIDToken, token.RefreshToken, claimsJSON)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
