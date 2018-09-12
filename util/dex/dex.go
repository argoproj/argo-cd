package dex

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/coreos/dex/api"
	oidc "github.com/coreos/go-oidc"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	// DexReverseProxyAddr is the address of the Dex OIDC server, which we run a reverse proxy against
	DexReverseProxyAddr = "http://dex-server:5556"
	// DexgRPCAPIAddr is the address to the Dex gRPC API server for managing dex. This is assumed to run
	// locally (as a sidecar)
	DexgRPCAPIAddr = "localhost:5557"
)

var messageRe = regexp.MustCompile(`<p>(.*)([\s\S]*?)<\/p>`)

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
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == 500 {
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			err = resp.Body.Close()
			if err != nil {
				return err
			}
			var message string
			matches := messageRe.FindSubmatch(b)
			if len(matches) > 1 {
				message = html.UnescapeString(string(matches[1]))
			} else {
				message = "Unknown error"
			}
			resp.ContentLength = 0
			resp.Header.Set("Content-Length", strconv.Itoa(0))
			resp.Header.Set("Location", fmt.Sprintf("/login?sso_error=%s", url.QueryEscape(message)))
			resp.StatusCode = http.StatusSeeOther
			return nil
		}
		return nil
	}
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

type ClientApp struct {
	// OAuth2 client ID of this application (e.g. argo-cd)
	clientID string
	// OAuth2 client secret of this application
	clientSecret string
	// Callback URL for OAuth2 responses (e.g. https://argocd.example.com/auth/callback)
	redirectURI string
	// URL of the issuer (e.g. https://argocd.example.com/api/dex)
	issuerURL string
	// client is the HTTP client which is used to query the IDp
	client *http.Client
	// secureCookie indicates if the cookie should be set with the Secure flag, meaning it should
	// only ever be sent over HTTPS. This value is inferred by the scheme of the redirectURI.
	secureCookie bool
	// settings holds ArgoCD settings
	settings *settings.ArgoCDSettings
	// sessionMgr holds an ArgoCD session manager
	sessionMgr *session.SessionManager
	// states holds temporary nonce tokens to which hold application state values
	// See http://tools.ietf.org/html/rfc6749#section-10.12 for more info.
	states cache.Cache
}

type appState struct {
	// ReturnURL is the URL in which to redirect a user back to after completing an OAuth2 login
	ReturnURL string `json:"returnURL"`
}

// NewClientApp will register the ArgoCD client app in Dex and return an object which has HTTP
// handlers for handling the HTTP responses for login and callback
func NewClientApp(settings *settings.ArgoCDSettings, sessionMgr *session.SessionManager) (*ClientApp, error) {
	log.Infof("Creating client app (%s)", common.ArgoCDClientAppID)
	a := ClientApp{
		clientID:     common.ArgoCDClientAppID,
		clientSecret: settings.OAuth2ClientSecret(),
		redirectURI:  settings.RedirectURL(),
		issuerURL:    settings.IssuerURL(),
	}
	u, err := url.Parse(settings.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redirect-uri: %v", err)
	}
	tlsConfig := settings.TLSConfig()
	if tlsConfig != nil {
		tlsConfig.InsecureSkipVerify = true
	}
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
	// NOTE: if we ever have replicas of ArgoCD, this needs to switch to Redis cache
	a.states = cache.NewInMemoryCache(3 * time.Minute)
	a.secureCookie = bool(u.Scheme == "https")
	a.settings = settings
	a.sessionMgr = sessionMgr
	return &a, nil
}

func (a *ClientApp) oauth2Config(scopes []string) (*oauth2.Config, error) {
	provider, err := a.sessionMgr.OIDCProvider()
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
		RedirectURL:  a.redirectURI,
	}, nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// generateAppState creates an app state nonce
func (a *ClientApp) generateAppState(returnURL string) string {
	randStr := randString(10)
	if returnURL == "" {
		returnURL = "/"
	}
	err := a.states.Set(&cache.Item{
		Key: randStr,
		Object: &appState{
			ReturnURL: returnURL,
		},
	})
	if err != nil {
		// This should never happen with the in-memory cache
		log.Errorf("Failed to set app state: %v", err)
	}
	return randStr
}

func (a *ClientApp) verifyAppState(state string) (*appState, error) {
	var aState appState
	err := a.states.Get(state, &aState)
	if err != nil {
		if err == cache.ErrCacheMiss {
			return nil, fmt.Errorf("unknown app state %s", state)
		} else {
			return nil, fmt.Errorf("failed to verify app state %s: %v", state, err)
		}
	}
	// TODO: purge the state string from the cache so that it is a true nonce
	return &aState, nil
}

func (a *ClientApp) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var opts []oauth2.AuthCodeOption
	returnURL := r.FormValue("return_url")
	scopes := []string{"openid", "profile", "email", "groups"}
	appState := a.generateAppState(returnURL)
	if r.FormValue("offline_access") != "yes" {
		// no-op
	} else if a.sessionMgr.OfflineAsScope() {
		scopes = append(scopes, "offline_access")
	} else {
		opts = append(opts, oauth2.AccessTypeOffline)
	}
	oauth2Config, err := a.oauth2Config(scopes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authCodeURL := oauth2Config.AuthCodeURL(appState, opts...)
	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

func (a *ClientApp) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		token     *oauth2.Token
		returnURL string
	)

	ctx := oidc.ClientContext(r.Context(), a.client)
	oauth2Config, err := a.oauth2Config(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
		var aState *appState
		aState, err = a.verifyAppState(r.FormValue("state"))
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
			return
		}
		returnURL = aState.ReturnURL
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
			Expiry:       time.Now().UTC().Add(-time.Hour),
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

	idToken, err := a.verify(rawIDToken)
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
	flags := []string{"path=/"}
	if a.secureCookie {
		flags = append(flags, "Secure")
	}
	cookie := session.MakeCookieMetadata(common.AuthCookieName, rawIDToken, flags...)
	w.Header().Set("Set-Cookie", cookie)
	log.Infof("Web login successful claims: %v", claims)
	if os.Getenv(common.EnvVarSSODebug) == "1" {
		claimsJSON, _ := json.MarshalIndent(claims, "", "  ")
		renderToken(w, a.redirectURI, rawIDToken, token.RefreshToken, claimsJSON)
	} else {
		http.Redirect(w, r, returnURL, http.StatusSeeOther)
	}
}

func (a *ClientApp) verify(tokenString string) (*oidc.IDToken, error) {
	provider, err := a.sessionMgr.OIDCProvider()
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: a.clientID})
	return verifier.Verify(context.Background(), tokenString)
}
