package oidc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/server/settings/oidc"
	"github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/crypto"
	"github.com/argoproj/argo-cd/v3/util/dex"

	httputil "github.com/argoproj/argo-cd/v3/util/http"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	"github.com/argoproj/argo-cd/v3/util/rand"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

var ErrInvalidRedirectURL = errors.New("invalid return URL")

const (
	GrantTypeAuthorizationCode  = "authorization_code"
	GrantTypeImplicit           = "implicit"
	ResponseTypeCode            = "code"
	UserInfoResponseCachePrefix = "userinfo_response"
	AccessTokenCachePrefix      = "access_token"
)

// OIDCConfiguration holds a subset of interested fields from the OIDC configuration spec
type OIDCConfiguration struct {
	Issuer                 string   `json:"issuer"`
	ScopesSupported        []string `json:"scopes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
	GrantTypesSupported    []string `json:"grant_types_supported,omitempty"`
}

type ClaimsRequest struct {
	IDToken map[string]*oidc.Claim `json:"id_token"`
}

type ClientApp struct {
	// OAuth2 client ID of this application (e.g. argo-cd)
	clientID string
	// OAuth2 client secret of this application
	clientSecret string
	// Use Proof Key for Code Exchange (PKCE)
	usePKCE bool
	// Use Azure Workload Identity for clientID auth instead of clientSecret
	useAzureWorkloadIdentity bool
	// Callback URL for OAuth2 responses (e.g. https://argocd.example.com/auth/callback)
	redirectURI string
	// URL of the issuer (e.g. https://argocd.example.com/api/dex)
	issuerURL string
	// The URL endpoint at which the ArgoCD server is accessed.
	baseHRef string
	// client is the HTTP client which is used to query the IDp
	client *http.Client
	// secureCookie indicates if the cookie should be set with the Secure flag, meaning it should
	// only ever be sent over HTTPS. This value is inferred by the scheme of the redirectURI.
	secureCookie bool
	// settings holds Argo CD settings
	settings *settings.ArgoCDSettings
	// encryptionKey holds server encryption key
	encryptionKey []byte
	// provider is the OIDC provider
	provider Provider
	// clientCache represent a cache of sso artifact
	clientCache cache.CacheClient
	// properties for azure workload identity.
	azure azureApp
}

type azureApp struct {
	// federated azure token for the service account
	assertion string
	// expiry of the token
	expires time.Time
	// mutex for parallelism for reading the token
	mtx *sync.RWMutex
}

func GetScopesOrDefault(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{"openid", "profile", "email", "groups"}
	}
	return scopes
}

// NewClientApp will register the Argo CD client app (either via Dex or external OIDC) and return an
// object which has HTTP handlers for handling the HTTP responses for login and callback
func NewClientApp(settings *settings.ArgoCDSettings, dexServerAddr string, dexTLSConfig *dex.DexTLSConfig, baseHRef string, cacheClient cache.CacheClient) (*ClientApp, error) {
	redirectURL, err := settings.RedirectURL()
	if err != nil {
		return nil, err
	}
	encryptionKey, err := settings.GetServerEncryptionKey()
	if err != nil {
		return nil, err
	}
	a := ClientApp{
		clientID:                 settings.OAuth2ClientID(),
		clientSecret:             settings.OAuth2ClientSecret(),
		usePKCE:                  settings.OAuth2UsePKCE(),
		useAzureWorkloadIdentity: settings.UseAzureWorkloadIdentity(),
		redirectURI:              redirectURL,
		issuerURL:                settings.IssuerURL(),
		baseHRef:                 baseHRef,
		encryptionKey:            encryptionKey,
		clientCache:              cacheClient,
		azure:                    azureApp{mtx: &sync.RWMutex{}},
	}
	log.Infof("Creating client app (%s)", a.clientID)
	u, err := url.Parse(settings.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redirect-uri: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	a.client = &http.Client{
		Transport: transport,
	}

	if settings.DexConfig != "" && settings.OIDCConfigRAW == "" {
		transport.TLSClientConfig = dex.TLSConfig(dexTLSConfig)
		addrWithProto := dex.DexServerAddressWithProtocol(dexServerAddr, dexTLSConfig)
		a.client.Transport = dex.NewDexRewriteURLRoundTripper(addrWithProto, a.client.Transport)
	} else {
		transport.TLSClientConfig = settings.OIDCTLSConfig()
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

func (a *ClientApp) oauth2Config(request *http.Request, scopes []string) (*oauth2.Config, error) {
	endpoint, err := a.provider.Endpoint()
	if err != nil {
		return nil, err
	}
	redirectURL, err := a.settings.RedirectURLForRequest(request)
	if err != nil {
		log.Warnf("Unable to find ArgoCD URL from request, falling back to configured redirect URI: %v", err)
		redirectURL = a.redirectURI
	}

	return &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint:     *endpoint,
		Scopes:       scopes,
		RedirectURL:  redirectURL,
	}, nil
}

// generateAppState creates an app state nonce
func (a *ClientApp) generateAppState(returnURL string, pkceVerifier string, w http.ResponseWriter) (string, error) {
	// According to the spec (https://www.rfc-editor.org/rfc/rfc6749#section-10.10), this must be guessable with
	// probability <= 2^(-128). The following call generates one of 52^24 random strings, ~= 2^136 possibilities.
	randStr, err := rand.String(24)
	if err != nil {
		return "", fmt.Errorf("failed to generate app state: %w", err)
	}
	if returnURL == "" {
		returnURL = a.baseHRef
	}
	cookieValue := fmt.Sprintf("%s\n%s\n%s", randStr, returnURL, pkceVerifier)
	if encrypted, err := crypto.Encrypt([]byte(cookieValue), a.encryptionKey); err != nil {
		return "", err
	} else {
		cookieValue = hex.EncodeToString(encrypted)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     common.StateCookieName,
		Value:    cookieValue,
		Expires:  time.Now().Add(common.StateCookieMaxAge),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.secureCookie,
	})
	return randStr, nil
}

func (a *ClientApp) verifyAppState(r *http.Request, w http.ResponseWriter, state string) (string, string, error) {
	c, err := r.Cookie(common.StateCookieName)
	if err != nil {
		return "", "", err
	}
	val, err := hex.DecodeString(c.Value)
	if err != nil {
		return "", "", err
	}
	val, err = crypto.Decrypt(val, a.encryptionKey)
	if err != nil {
		return "", "", err
	}
	cookieVal := string(val)
	redirectURL := a.baseHRef
	pkceVerifier := ""
	parts := strings.SplitN(cookieVal, "\n", 3)
	if len(parts) > 1 && parts[1] != "" {
		if !isValidRedirectURL(parts[1],
			append([]string{a.settings.URL, a.baseHRef}, a.settings.AdditionalURLs...)) {
			sanitizedURL := parts[1]
			if len(sanitizedURL) > 100 {
				sanitizedURL = sanitizedURL[:100]
			}
			log.Warnf("Failed to verify app state - got invalid redirectURL %q", sanitizedURL)
			return "", "", fmt.Errorf("failed to verify app state: %w", ErrInvalidRedirectURL)
		}
		redirectURL = parts[1]
	}
	if len(parts) > 2 {
		pkceVerifier = parts[2]
	}
	if parts[0] != state {
		return "", "", fmt.Errorf("invalid state in '%s' cookie", common.AuthCookieName)
	}
	// set empty cookie to clear it
	http.SetCookie(w, &http.Cookie{
		Name:     common.StateCookieName,
		Value:    "",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.secureCookie,
	})
	return redirectURL, pkceVerifier, nil
}

// isValidRedirectURL checks whether the given redirectURL matches on of the
// allowed URLs to redirect to.
//
// In order to be considered valid,the protocol and host (including port) have
// to match and if allowed path is not "/", redirectURL's path must be within
// allowed URL's path.
func isValidRedirectURL(redirectURL string, allowedURLs []string) bool {
	if redirectURL == "" {
		return true
	}
	r, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}
	// We consider empty path the same as "/" for redirect URL
	if r.Path == "" {
		r.Path = "/"
	}
	// Prevent CRLF in the redirectURL
	if strings.ContainsAny(r.Path, "\r\n") {
		return false
	}
	for _, baseURL := range allowedURLs {
		b, err := url.Parse(baseURL)
		if err != nil {
			continue
		}
		// We consider empty path the same as "/" for allowed URL
		if b.Path == "" {
			b.Path = "/"
		}
		// scheme and host are mandatory to match.
		if b.Scheme == r.Scheme && b.Host == r.Host {
			// If path of redirectURL and allowedURL match, redirectURL is allowed
			// if b.Path == r.Path {
			//	 return true
			// }
			// If path of redirectURL is within allowed URL's path, redirectURL is allowed
			if strings.HasPrefix(path.Clean(r.Path), b.Path) {
				return true
			}
		}
	}
	// No match - redirect URL is not allowed
	return false
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
	pkceVerifier := ""
	var opts []oauth2.AuthCodeOption
	if config := a.settings.OIDCConfig(); config != nil {
		scopes = GetScopesOrDefault(config.RequestedScopes)
		opts = AppendClaimsAuthenticationRequestParameter(opts, config.RequestedIDTokenClaims)
	} else if a.settings.IsDexConfigured() {
		scopes = append(GetScopesOrDefault(nil), common.DexFederatedScope)
	}

	oauth2Config, err := a.oauth2Config(r, scopes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	returnURL := r.FormValue("return_url")
	// Check if return_url is valid, otherwise abort processing (see https://github.com/argoproj/argo-cd/pull/4780)
	if !isValidRedirectURL(returnURL, append([]string{a.settings.URL}, a.settings.AdditionalURLs...)) {
		http.Error(w, "Invalid redirect URL: the protocol and host (including port) must match and the path must be within allowed URLs if provided", http.StatusBadRequest)
		return
	}
	if a.usePKCE {
		pkceVerifier = oauth2.GenerateVerifier()
		opts = append(opts, oauth2.S256ChallengeOption(pkceVerifier))
	}
	stateNonce, err := a.generateAppState(returnURL, pkceVerifier, w)
	if err != nil {
		log.Errorf("Failed to initiate login flow: %v", err)
		http.Error(w, "Failed to initiate login flow", http.StatusInternalServerError)
		return
	}
	grantType := InferGrantType(oidcConf)
	var url string
	switch grantType {
	case GrantTypeAuthorizationCode:
		url = oauth2Config.AuthCodeURL(stateNonce, opts...)
	case GrantTypeImplicit:
		url, err = ImplicitFlowURL(oauth2Config, stateNonce, opts...)
		if err != nil {
			log.Errorf("Failed to initiate implicit login flow: %v", err)
			http.Error(w, "Failed to initiate implicit login flow", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("Unsupported grant type: %v", grantType), http.StatusInternalServerError)
		return
	}
	log.Infof("Performing %s flow login: %s", grantType, url)
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// getFederatedServiceAccountToken returns the specified file's content, which is expected to be Federated Kubernetes service account token.
// Kubernetes is responsible for updating the file as service account tokens expire.
// Azure Workload Identity mutation webhook will set the environment variable AZURE_FEDERATED_TOKEN_FILE
// Content of this file will contain a federated token which can be used in assertion with Microsoft Entra Application.
func (a *azureApp) getFederatedServiceAccountToken(context.Context) (string, error) {
	file, ok := os.LookupEnv("AZURE_FEDERATED_TOKEN_FILE")
	if file == "" || !ok {
		return "", errors.New("AZURE_FEDERATED_TOKEN_FILE env variable not found, make sure workload identity is enabled on the cluster")
	}

	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		return "", errors.New("AZURE_FEDERATED_TOKEN_FILE specified file does not exist")
	}

	a.mtx.RLock()
	if a.expires.Before(time.Now()) {
		// ensure only one goroutine at a time updates the assertion
		a.mtx.RUnlock()
		a.mtx.Lock()
		defer a.mtx.Unlock()
		// double check because another goroutine may have acquired the write lock first and done the update
		if now := time.Now(); a.expires.Before(now) {
			content, err := os.ReadFile(file)
			if err != nil {
				return "", err
			}
			a.assertion = string(content)
			// Kubernetes rotates service account tokens when they reach 80% of their total TTL. The shortest TTL
			// is 1 hour. That implies the token we just read is valid for at least 12 minutes (20% of 1 hour),
			// but we add some margin for safety.
			a.expires = now.Add(10 * time.Minute)
		}
	} else {
		defer a.mtx.RUnlock()
	}
	return a.assertion, nil
}

// HandleCallback is the callback handler for an OAuth2 login flow
func (a *ClientApp) HandleCallback(w http.ResponseWriter, r *http.Request) {
	oauth2Config, err := a.oauth2Config(r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Infof("Callback: %s", r.URL)
	if errMsg := r.FormValue("error"); errMsg != "" {
		errorDesc := r.FormValue("error_description")
		http.Error(w, html.EscapeString(errMsg)+": "+html.EscapeString(errorDesc), http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")
	state := r.FormValue("state")
	if code == "" {
		// If code was not given, it implies implicit flow
		a.handleImplicitFlow(r, w, state)
		return
	}
	returnURL, pkceVerifier, err := a.verifyAppState(r, w, state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := gooidc.ClientContext(r.Context(), a.client)
	options := []oauth2.AuthCodeOption{}

	if a.useAzureWorkloadIdentity {
		clientAssertion, err := a.azure.getFederatedServiceAccountToken(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to generate client assertion: %v", err), http.StatusInternalServerError)
			return
		}

		options = []oauth2.AuthCodeOption{
			oauth2.SetAuthURLParam("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"),
			oauth2.SetAuthURLParam("client_assertion", clientAssertion),
		}
	}

	if a.usePKCE {
		options = append(options, oauth2.VerifierOption(pkceVerifier))
	}

	token, err := oauth2Config.Exchange(ctx, code, options...)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	idTokenRAW, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in token response", http.StatusInternalServerError)
		return
	}

	idToken, err := a.provider.Verify(idTokenRAW, a.settings)
	if err != nil {
		log.Warnf("Failed to verify token: %s", err)
		http.Error(w, common.TokenVerificationError, http.StatusInternalServerError)
		return
	}
	path := "/"
	if a.baseHRef != "" {
		path = strings.TrimRight(strings.TrimLeft(a.baseHRef, "/"), "/")
	}
	cookiePath := "path=/" + path
	flags := []string{cookiePath, "SameSite=lax", "httpOnly"}
	if a.secureCookie {
		flags = append(flags, "Secure")
	}
	var claims jwt.MapClaims
	err = idToken.Claims(&claims)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// save the accessToken in memory for later use
	encToken, err := crypto.Encrypt([]byte(token.AccessToken), a.encryptionKey)
	if err != nil {
		claimsJSON, _ := json.Marshal(claims)
		http.Error(w, "failed encrypting token", http.StatusInternalServerError)
		log.Errorf("cannot encrypt accessToken: %v (claims=%s)", err, claimsJSON)
		return
	}
	sub := jwtutil.StringField(claims, "sub")
	err = a.clientCache.Set(&cache.Item{
		Key:    formatAccessTokenCacheKey(sub),
		Object: encToken,
		CacheActionOpts: cache.CacheActionOpts{
			Expiration: getTokenExpiration(claims),
		},
	})
	if err != nil {
		claimsJSON, _ := json.Marshal(claims)
		http.Error(w, fmt.Sprintf("claims=%s, err=%v", claimsJSON, err), http.StatusInternalServerError)
		return
	}

	if idTokenRAW != "" {
		cookies, err := httputil.MakeCookieMetadata(common.AuthCookieName, idTokenRAW, flags...)
		if err != nil {
			claimsJSON, _ := json.Marshal(claims)
			http.Error(w, fmt.Sprintf("claims=%s, err=%v", claimsJSON, err), http.StatusInternalServerError)
			return
		}

		for _, cookie := range cookies {
			w.Header().Add("Set-Cookie", cookie)
		}
	}

	claimsJSON, _ := json.Marshal(claims)
	log.Infof("Web login successful. Claims: %s", claimsJSON)
	if os.Getenv(common.EnvVarSSODebug) == "1" {
		claimsJSON, _ := json.MarshalIndent(claims, "", "  ")
		renderToken(w, a.redirectURI, idTokenRAW, token.RefreshToken, claimsJSON)
	} else {
		http.Redirect(w, r, returnURL, http.StatusSeeOther)
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
func (a *ClientApp) handleImplicitFlow(r *http.Request, w http.ResponseWriter, state string) {
	type implicitFlowValues struct {
		CookieName string
		ReturnURL  string
	}
	vals := implicitFlowValues{
		CookieName: common.AuthCookieName,
	}
	if state != "" {
		// Not using pkceVerifier, since PKCE is not supported in implicit flow.
		returnURL, _, err := a.verifyAppState(r, w, state)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		vals.ReturnURL = returnURL
	}
	renderTemplate(w, implicitFlowTmpl, vals)
}

// ImplicitFlowURL is an adaptation of oauth2.Config::AuthCodeURL() which returns a URL
// appropriate for an OAuth2 implicit login flow (as opposed to authorization code flow).
func ImplicitFlowURL(c *oauth2.Config, state string, opts ...oauth2.AuthCodeOption) (string, error) {
	opts = append(opts, oauth2.SetAuthURLParam("response_type", "id_token"))
	randString, err := rand.String(24)
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce for implicit flow URL: %w", err)
	}
	opts = append(opts, oauth2.SetAuthURLParam("nonce", randString))
	return c.AuthCodeURL(state, opts...), nil
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
func InferGrantType(oidcConf *OIDCConfiguration) string {
	// Check the supported response types. If the list contains the response type 'code',
	// then grant type is 'authorization_code'. This is preferred over the implicit
	// grant type since refresh tokens cannot be issued that way.
	for _, supportedType := range oidcConf.ResponseTypesSupported {
		if supportedType == ResponseTypeCode {
			return GrantTypeAuthorizationCode
		}
	}

	// Assume implicit otherwise
	return GrantTypeImplicit
}

// AppendClaimsAuthenticationRequestParameter appends a OIDC claims authentication request parameter
// to `opts` with the `requestedClaims`
func AppendClaimsAuthenticationRequestParameter(opts []oauth2.AuthCodeOption, requestedClaims map[string]*oidc.Claim) []oauth2.AuthCodeOption {
	if len(requestedClaims) == 0 {
		return opts
	}
	log.Infof("RequestedClaims: %s\n", requestedClaims)
	claimsRequestParameter, err := createClaimsAuthenticationRequestParameter(requestedClaims)
	if err != nil {
		log.Errorf("Failed to create OIDC claims authentication request parameter from config: %s", err)
		return opts
	}
	return append(opts, claimsRequestParameter)
}

func createClaimsAuthenticationRequestParameter(requestedClaims map[string]*oidc.Claim) (oauth2.AuthCodeOption, error) {
	claimsRequest := ClaimsRequest{IDToken: requestedClaims}
	claimsRequestRAW, err := json.Marshal(claimsRequest)
	if err != nil {
		return nil, err
	}
	return oauth2.SetAuthURLParam("claims", string(claimsRequestRAW)), nil
}

// GetUserInfo queries the IDP userinfo endpoint for claims
func (a *ClientApp) GetUserInfo(actualClaims jwt.MapClaims, issuerURL, userInfoPath string) (jwt.MapClaims, bool, error) {
	sub := jwtutil.StringField(actualClaims, "sub")
	var claims jwt.MapClaims
	var encClaims []byte

	// in case we got it in the cache, we just return the item
	clientCacheKey := formatUserInfoResponseCacheKey(sub)
	if err := a.clientCache.Get(clientCacheKey, &encClaims); err == nil {
		claimsRaw, err := crypto.Decrypt(encClaims, a.encryptionKey)
		if err != nil {
			log.Errorf("decrypting the cached claims failed (sub=%s): %s", sub, err)
		} else {
			err = json.Unmarshal(claimsRaw, &claims)
			if err == nil {
				// return the cached claims since they are not yet expired, were successfully decrypted and unmarshaled
				return claims, false, nil
			}
			log.Errorf("cannot unmarshal cached claims structure: %s", err)
		}
	}

	// check if the accessToken for the user is still present
	var encAccessToken []byte
	err := a.clientCache.Get(formatAccessTokenCacheKey(sub), &encAccessToken)
	// without an accessToken we can't query the user info endpoint
	// thus the user needs to reauthenticate for argocd to get a new accessToken
	if errors.Is(err, cache.ErrCacheMiss) {
		return claims, true, fmt.Errorf("no accessToken for %s: %w", sub, err)
	} else if err != nil {
		return claims, true, fmt.Errorf("couldn't read accessToken from cache for %s: %w", sub, err)
	}

	accessToken, err := crypto.Decrypt(encAccessToken, a.encryptionKey)
	if err != nil {
		return claims, true, fmt.Errorf("couldn't decrypt accessToken for %s: %w", sub, err)
	}

	url := issuerURL + userInfoPath
	request, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		err = fmt.Errorf("failed creating new http request: %w", err)
		return claims, false, err
	}

	bearer := fmt.Sprintf("Bearer %s", accessToken)
	request.Header.Set("Authorization", bearer)

	response, err := a.client.Do(request)
	if err != nil {
		return claims, false, fmt.Errorf("failed to query userinfo endpoint of IDP: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return claims, true, err
	}

	// according to https://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponseValidation
	// the response should be validated
	header := response.Header.Get("content-type")
	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return claims, false, fmt.Errorf("got error reading response body: %w", err)
	}
	switch header {
	case "application/jwt":
		// if body is JWT, first validate it before extracting claims
		idToken, err := a.provider.Verify(string(rawBody), a.settings)
		if err != nil {
			return claims, false, fmt.Errorf("user info response in jwt format not valid: %w", err)
		}
		err = idToken.Claims(claims)
		if err != nil {
			return claims, false, fmt.Errorf("cannot get claims from userinfo jwt: %w", err)
		}
	default:
		// if body is json, unsigned and unencrypted claims can be deserialized
		err = json.Unmarshal(rawBody, &claims)
		if err != nil {
			return claims, false, fmt.Errorf("failed to decode response body to struct: %w", err)
		}
	}

	// in case response was successfully validated and there was no error, put item in cache
	// but first let's determine the expiry of the cache
	var cacheExpiry time.Duration
	settingExpiry := a.settings.UserInfoCacheExpiration()
	tokenExpiry := getTokenExpiration(claims)

	// only use configured expiry if the token lives longer and the expiry is configured
	// if the token has no expiry, use the expiry of the actual token
	// otherwise use the expiry of the token
	switch {
	case settingExpiry < tokenExpiry && settingExpiry != 0:
		cacheExpiry = settingExpiry
	case tokenExpiry < 0:
		cacheExpiry = getTokenExpiration(actualClaims)
	default:
		cacheExpiry = tokenExpiry
	}

	rawClaims, err := json.Marshal(claims)
	if err != nil {
		return claims, false, fmt.Errorf("couldn't marshal claim to json: %w", err)
	}
	encClaims, err = crypto.Encrypt(rawClaims, a.encryptionKey)
	if err != nil {
		return claims, false, fmt.Errorf("couldn't encrypt user info response: %w", err)
	}

	err = a.clientCache.Set(&cache.Item{
		Key:    clientCacheKey,
		Object: encClaims,
		CacheActionOpts: cache.CacheActionOpts{
			Expiration: cacheExpiry,
		},
	})
	if err != nil {
		return claims, false, fmt.Errorf("couldn't put item to cache: %w", err)
	}

	return claims, false, nil
}

// getTokenExpiration returns a time.Duration until the token expires
func getTokenExpiration(claims jwt.MapClaims) time.Duration {
	// get duration until token expires
	exp := jwtutil.Float64Field(claims, "exp")
	tm := time.Unix(int64(exp), 0)
	tokenExpiry := time.Until(tm)
	return tokenExpiry
}

// formatUserInfoResponseCacheKey returns the key which is used to store userinfo of user in cache
func formatUserInfoResponseCacheKey(sub string) string {
	return fmt.Sprintf("%s_%s", UserInfoResponseCachePrefix, sub)
}

// formatAccessTokenCacheKey returns the key which is used to store the accessToken of a user in cache
func formatAccessTokenCacheKey(sub string) string {
	return fmt.Sprintf("%s_%s", AccessTokenCachePrefix, sub)
}
