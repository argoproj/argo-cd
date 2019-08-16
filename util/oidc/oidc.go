package oidc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/dex"
	httputil "github.com/argoproj/argo-cd/util/http"
	"github.com/argoproj/argo-cd/util/rand"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	GrantTypeAuthorizationCode = "authorization_code"
	GrantTypeImplicit          = "implicit"
	ResponseTypeCode           = "code"
)

// OIDCConfiguration holds a subset of interested fields from the OIDC configuration spec
type OIDCConfiguration struct {
	Issuer                 string   `json:"issuer"`
	ScopesSupported        []string `json:"scopes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
	GrantTypesSupported    []string `json:"grant_types_supported,omitempty"`
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
	// settings holds Argo CD settings
	settings *settings.ArgoCDSettings
	// provider is the OIDC provider
	provider Provider
	// cache holds temporary nonce tokens to which hold application state values
	// See http://tools.ietf.org/html/rfc6749#section-10.12 for more info.
	cache *cache.Cache
}

func GetScopesOrDefault(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{"openid", "profile", "email", "groups"}
	}
	return scopes
}

// NewClientApp will register the Argo CD client app (either via Dex or external OIDC) and return an
// object which has HTTP handlers for handling the HTTP responses for login and callback
func NewClientApp(settings *settings.ArgoCDSettings, cache *cache.Cache, dexServerAddr string) (*ClientApp, error) {
	a := ClientApp{
		clientID:     settings.OAuth2ClientID(),
		clientSecret: settings.OAuth2ClientSecret(),
		redirectURI:  settings.RedirectURL(),
		issuerURL:    settings.IssuerURL(),
		cache:        cache,
	}
	log.Infof("Creating client app (%s)", a.clientID)
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
	if settings.DexConfig != "" && settings.OIDCConfigRAW == "" {
		a.client.Transport = dex.NewDexRewriteURLRoundTripper(dexServerAddr, a.client.Transport)
	}
	if os.Getenv(common.EnvVarSSODebug) == "1" {
		a.client.Transport = httputil.DebugTransport{T: a.client.Transport}
	}

	a.provider = NewOIDCProvider(a.issuerURL, a.client)
	// NOTE: if we ever have replicas of Argo CD, this needs to switch to Redis cache
	a.secureCookie = bool(u.Scheme == "https")
	a.settings = settings
	return &a, nil
}

func (a *ClientApp) oauth2Config(scopes []string) (*oauth2.Config, error) {
	endpoint, err := a.provider.Endpoint()
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint:     *endpoint,
		Scopes:       scopes,
		RedirectURL:  a.redirectURI,
	}, nil
}

// generateAppState creates an app state nonce
func (a *ClientApp) generateAppState(returnURL string) string {
	randStr := rand.RandString(10)
	if returnURL == "" {
		returnURL = "/"
	}
	err := a.cache.SetOIDCState(randStr, &cache.OIDCState{ReturnURL: returnURL})
	if err != nil {
		// This should never happen with the in-memory cache
		log.Errorf("Failed to set app state: %v", err)
	}
	return randStr
}

func (a *ClientApp) verifyAppState(state string) (*cache.OIDCState, error) {
	res, err := a.cache.GetOIDCState(state)
	if err != nil {
		if err == cache.ErrCacheMiss {
			return nil, fmt.Errorf("unknown app state %s", state)
		} else {
			return nil, fmt.Errorf("failed to verify app state %s: %v", state, err)
		}
	}

	_ = a.cache.SetOIDCState(state, nil)
	return res, nil
}

// HandleLogin formulates the proper OAuth2 URL (auth code or implicit) and redirects the user to
// the IDp login & consent page
func (a *ClientApp) HandleLogin(w http.ResponseWriter, r *http.Request) {
	oidcConf, err := a.provider.ParseConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	scopes := make([]string, 0)
	if config := a.settings.OIDCConfig(); config != nil {
		scopes = config.RequestedScopes
	}
	oauth2Config, err := a.oauth2Config(GetScopesOrDefault(scopes))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	returnURL := r.FormValue("return_url")
	stateNonce := a.generateAppState(returnURL)
	grantType := InferGrantType(oauth2Config, oidcConf)
	var url string
	switch grantType {
	case GrantTypeAuthorizationCode:
		url = oauth2Config.AuthCodeURL(stateNonce)
	case GrantTypeImplicit:
		url = ImplicitFlowURL(oauth2Config, stateNonce)
	default:
		http.Error(w, fmt.Sprintf("Unsupported grant type: %v", grantType), http.StatusInternalServerError)
		return
	}
	log.Infof("Performing %s flow login: %s", grantType, url)
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// HandleCallback is the callback handler for an OAuth2 login flow
func (a *ClientApp) HandleCallback(w http.ResponseWriter, r *http.Request) {
	oauth2Config, err := a.oauth2Config(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Infof("Callback: %s", r.URL)
	if errMsg := r.FormValue("error"); errMsg != "" {
		http.Error(w, errMsg+": "+r.FormValue("error_description"), http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")
	state := r.FormValue("state")
	if code == "" {
		// If code was not given, it implies implicit flow
		a.handleImplicitFlow(w, state)
		return
	}
	appState, err := a.verifyAppState(state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := gooidc.ClientContext(r.Context(), a.client)
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		return
	}
	idTokenRAW, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in token response", http.StatusInternalServerError)
		return
	}
	idToken, err := a.provider.Verify(a.clientID, idTokenRAW)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid session token: %v", err), http.StatusInternalServerError)
		return
	}
	flags := []string{"path=/"}
	if a.secureCookie {
		flags = append(flags, "Secure")
	}
	var claims jwt.MapClaims
	err = idToken.Claims(&claims)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cookie, err := httputil.MakeCookieMetadata(common.AuthCookieName, idTokenRAW, flags...)
	if err != nil {
		claimsJSON, _ := json.Marshal(claims)
		http.Error(w, fmt.Sprintf("claims=%s, err=%v", claimsJSON, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Set-Cookie", cookie)

	claimsJSON, _ := json.Marshal(claims)
	log.Infof("Web login successful. Claims: %s", claimsJSON)
	if os.Getenv(common.EnvVarSSODebug) == "1" {
		claimsJSON, _ := json.MarshalIndent(claims, "", "  ")
		renderToken(w, a.redirectURI, idTokenRAW, token.RefreshToken, claimsJSON)
	} else {
		http.Redirect(w, r, appState.ReturnURL, http.StatusSeeOther)
	}
}

var implicitFlowTmpl = template.Must(template.New("implicit.html").Parse(`<script>
var hash = window.location.hash.substr(1);
var result = hash.split('&').reduce(function (result, item) {
	var parts = item.split('=');
	result[parts[0]] = parts[1];
	return result;
}, {});
var idToken = result['id_token'];
var state = result['state'];
var returnURL = "{{ .ReturnURL }}";
if (state != "" && returnURL == "") {
	window.location.href = window.location.href.split("#")[0] + "?state=" + result['state'] + window.location.hash;
} else if (returnURL != "") {
	document.cookie = "{{ .CookieName }}=" + idToken + "; path=/";
	window.location.href = returnURL;
}
</script>`))

// handleImplicitFlow completes an implicit OAuth2 flow. The id_token and state will be contained
// in the URL fragment. The javascript client first redirects to the callback URL, supplying the
// state nonce for verification, as well as looking up the return URL. Once verified, the client
// stores the id_token from the fragment as a cookie. Finally it performs the final redirect back to
// the return URL.
func (a *ClientApp) handleImplicitFlow(w http.ResponseWriter, state string) {
	type implicitFlowValues struct {
		CookieName string
		ReturnURL  string
	}
	vals := implicitFlowValues{
		CookieName: common.AuthCookieName,
	}
	if state != "" {
		appState, err := a.verifyAppState(state)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		vals.ReturnURL = appState.ReturnURL
	}
	renderTemplate(w, implicitFlowTmpl, vals)
}

// ImplicitFlowURL is an adaptation of oauth2.Config::AuthCodeURL() which returns a URL
// appropriate for an OAuth2 implicit login flow (as opposed to authorization code flow).
func ImplicitFlowURL(c *oauth2.Config, state string, opts ...oauth2.AuthCodeOption) string {
	var buf bytes.Buffer
	buf.WriteString(c.Endpoint.AuthURL)
	v := url.Values{
		"response_type": {"id_token"},
		"nonce":         {rand.RandString(10)},
		"client_id":     {c.ClientID},
		"redirect_uri":  condVal(c.RedirectURL),
		"scope":         condVal(strings.Join(c.Scopes, " ")),
		"state":         condVal(state),
	}
	for _, opt := range opts {
		switch opt {
		case oauth2.AccessTypeOnline:
			v.Set("access_type", "online")
		case oauth2.AccessTypeOffline:
			v.Set("access_type", "offline")
		case oauth2.ApprovalForce:
			v.Set("approval_prompt", "force")
		}
	}
	if strings.Contains(c.Endpoint.AuthURL, "?") {
		buf.WriteByte('&')
	} else {
		buf.WriteByte('?')
	}
	buf.WriteString(v.Encode())
	return buf.String()
}

func condVal(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}

// OfflineAccess returns whether or not 'offline_access' is a supported scope
func OfflineAccess(scopes []string) bool {
	if len(scopes) == 0 {
		// scopes_supported is a "RECOMMENDED" discovery claim, not a required
		// one. If missing, assume that the provider follows the spec and has
		// an "offline_access" scope.
		return true
	}
	// See if scopes_supported has the "offline_access" scope.
	for _, scope := range scopes {
		if scope == gooidc.ScopeOfflineAccess {
			return true
		}
	}
	return false
}

// InferGrantType infers the proper grant flow depending on the OAuth2 client config and OIDC configuration.
// Returns either: "authorization_code" or "implicit"
func InferGrantType(oauth2conf *oauth2.Config, oidcConf *OIDCConfiguration) string {
	if oauth2conf.ClientSecret != "" {
		// If we know the client secret, we are using the 'authorization_code' flow
		return GrantTypeAuthorizationCode
	}
	if len(oidcConf.ResponseTypesSupported) == 1 && oidcConf.ResponseTypesSupported[0] == ResponseTypeCode {
		// If we don't have the secret, check the supported response types. If the list is a single
		// response type of type 'code', then grant type is 'authorization_code'. This is the Dex
		// case, which does not support implicit login flow (https://github.com/dexidp/dex/issues/1254)
		return GrantTypeAuthorizationCode
	}
	// If we don't have the client secret (e.g. SPA app), we can assume to be implicit
	return GrantTypeImplicit
}
