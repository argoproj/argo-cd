package session

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/common"
	passwordutil "github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/coreos/go-oidc"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SessionManager generates and validates JWT tokens for login sessions.
type SessionManager struct {
	settings *settings.ArgoCDSettings
	client   *http.Client
	provider *oidc.Provider

	// Does the provider use "offline_access" scope to request a refresh token
	// or does it use "access_type=offline" (e.g. Google)?
	offlineAsScope bool
}

const (
	// SessionManagerClaimsIssuer fills the "iss" field of the token.
	SessionManagerClaimsIssuer = "argocd"

	// invalidLoginError, for security purposes, doesn't say whether the username or password was invalid.  This does not mitigate the potential for timing attacks to determine which is which.
	invalidLoginError  = "Invalid username or password"
	blankPasswordError = "Blank passwords are not allowed"
	badUserError       = "Bad local superuser username"
)

// NewSessionManager creates a new session manager from ArgoCD settings
func NewSessionManager(settings *settings.ArgoCDSettings) *SessionManager {
	s := SessionManager{
		settings: settings,
	}
	tlsConfig := settings.TLSConfig()
	if tlsConfig != nil {
		tlsConfig.InsecureSkipVerify = true
	}
	s.client = &http.Client{
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
	if os.Getenv(common.EnvVarSSODebug) == "1" {
		s.client.Transport = debugTransport{s.client.Transport}
	}
	return &s
}

// Create creates a new token for a given subject (user) and returns it as a string.
// Passing a value of `0` for secondsBeforeExpiry creates a token that never expires.
func (mgr *SessionManager) Create(subject string, secondsBeforeExpiry int) (string, error) {
	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	now := time.Now().UTC()
	claims := jwt.StandardClaims{
		IssuedAt:  now.Unix(),
		Issuer:    SessionManagerClaimsIssuer,
		NotBefore: now.Unix(),
		Subject:   subject,
	}
	if secondsBeforeExpiry > 0 {
		expires := now.Add(time.Duration(secondsBeforeExpiry) * time.Second)
		claims.ExpiresAt = expires.Unix()
	}
	return mgr.signClaims(claims)
}

func (mgr *SessionManager) signClaims(claims jwt.Claims) (string, error) {
	log.Infof("Issuing claims: %v", claims)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(mgr.settings.ServerSignature)
}

// ReissueClaims re-issues and re-signs a new token signed by us, while preserving most of the claim values
func (mgr *SessionManager) ReissueClaims(claims jwt.MapClaims, secondsBeforeExpiry int) (string, error) {
	now := time.Now().UTC()
	newClaims := make(jwt.MapClaims)
	for k, v := range claims {
		newClaims[k] = v
	}
	newClaims["iss"] = SessionManagerClaimsIssuer
	newClaims["iat"] = now.Unix()
	newClaims["nbf"] = now.Unix()
	delete(newClaims, "exp")
	if secondsBeforeExpiry > 0 {
		expires := now.Add(time.Duration(secondsBeforeExpiry) * time.Second)
		claims["exp"] = expires.Unix()
	}
	return mgr.signClaims(newClaims)
}

// Parse tries to parse the provided string and returns the token claims for local superuser login.
func (mgr *SessionManager) Parse(tokenString string) (jwt.Claims, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	var claims jwt.MapClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return mgr.settings.ServerSignature, nil
	})
	if err != nil {
		return nil, err
	}

	issuedAt := time.Unix(int64(claims["iat"].(float64)), 0)
	if issuedAt.Before(mgr.settings.AdminPasswordMtime) {
		return nil, fmt.Errorf("Password for superuser has changed since token issued")
	}
	return token.Claims, nil
}

// VerifyUsernamePassword verifies if a username/password combo is correct
func (mgr *SessionManager) VerifyUsernamePassword(username, password string) error {
	if username != common.ArgoCDAdminUsername {
		return status.Errorf(codes.Unauthenticated, badUserError)
	}
	if password == "" {
		return status.Errorf(codes.Unauthenticated, blankPasswordError)
	}
	valid, _ := passwordutil.VerifyPassword(password, mgr.settings.AdminPasswordHash)
	if !valid {
		return status.Errorf(codes.Unauthenticated, invalidLoginError)
	}
	return nil
}

// VerifyToken verifies if a token is correct. Tokens can be issued either from us or by dex.
// We choose how to verify based on the issuer.
func (mgr *SessionManager) VerifyToken(tokenString string) (jwt.Claims, error) {
	parser := &jwt.Parser{
		SkipClaimsValidation: true,
	}
	var claims jwt.StandardClaims
	_, _, err := parser.ParseUnverified(tokenString, &claims)
	if err != nil {
		return nil, err
	}
	switch claims.Issuer {
	case SessionManagerClaimsIssuer:
		// ArgoCD signed token
		return mgr.Parse(tokenString)
	default:
		// Dex signed token
		provider, err := mgr.OIDCProvider()
		if err != nil {
			return nil, err
		}
		verifier := provider.Verifier(&oidc.Config{ClientID: claims.Audience})
		idToken, err := verifier.Verify(context.Background(), tokenString)
		if err != nil {
			return nil, err
		}
		var claims jwt.MapClaims
		err = idToken.Claims(&claims)
		return claims, err
	}
}

func stringFromMap(input map[string]interface{}, key string) string {
	if val, ok := input[key]; ok {
		if res, ok := val.(string); ok {
			return res
		}
	}
	return ""
}

func Username(ctx context.Context) string {
	if claims, ok := ctx.Value("claims").(*jwt.MapClaims); ok {
		mapClaims := *claims
		switch stringFromMap(mapClaims, "iss") {
		case SessionManagerClaimsIssuer:
			return stringFromMap(mapClaims, "sub")
		default:
			return stringFromMap(mapClaims, "email")
		}
	}
	return ""
}

// MakeCookieMetadata generates a string representing a Web cookie.  Yum!
func MakeCookieMetadata(key, value string, flags ...string) string {
	components := []string{
		fmt.Sprintf("%s=%s", key, value),
	}
	components = append(components, flags...)
	return strings.Join(components, "; ")
}

// OIDCProvider lazily initializes and returns the OIDC provider, querying the well known oidc
// configuration path (http://example-argocd.com/api/dex/.well-known/openid-configuration).
// We have to initialize the proviver lazily since ArgoCD is an OIDC client to itself, which
// presents a chicken-and-egg problem of (1) serving dex over HTTP, and (2) querying the OIDC
// provider (ourselves) to initialize the app.
func (mgr *SessionManager) OIDCProvider() (*oidc.Provider, error) {
	if mgr.provider != nil {
		return mgr.provider, nil
	}
	if !mgr.settings.IsSSOConfigured() {
		return nil, fmt.Errorf("SSO is not configured")
	}
	issuerURL := mgr.settings.IssuerURL()
	log.Infof("Initializing OIDC provider (issuer: %s)", issuerURL)
	ctx := oidc.ClientContext(context.Background(), mgr.client)
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("Failed to query provider %q: %v", issuerURL, err)
	}

	// Returns the scopes the provider supports
	// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
	var s struct {
		ScopesSupported []string `json:"scopes_supported"`
	}
	if err := provider.Claims(&s); err != nil {
		return nil, fmt.Errorf("Failed to parse provider scopes_supported: %v", err)
	}
	log.Infof("OpenID supported scopes: %v", s.ScopesSupported)
	if len(s.ScopesSupported) == 0 {
		// scopes_supported is a "RECOMMENDED" discovery claim, not a required
		// one. If missing, assume that the provider follows the spec and has
		// an "offline_access" scope.
		mgr.offlineAsScope = true
	} else {
		// See if scopes_supported has the "offline_access" scope.
		for _, scope := range s.ScopesSupported {
			if scope == oidc.ScopeOfflineAccess {
				mgr.offlineAsScope = true
				break
			}
		}
	}
	mgr.provider = provider
	return mgr.provider, nil
}

func (mgr *SessionManager) OfflineAsScope() bool {
	_, _ = mgr.OIDCProvider() // forces offlineAsScope to be determined
	return mgr.offlineAsScope
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
