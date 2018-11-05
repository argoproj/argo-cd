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

	"github.com/coreos/go-oidc"
	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/common"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	oidcutil "github.com/argoproj/argo-cd/util/oidc"
	passwordutil "github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/settings"
)

// SessionManager generates and validates JWT tokens for login sessions.
type SessionManager struct {
	settings *settings.ArgoCDSettings
	client   *http.Client
	provider *oidc.Provider
}

const (
	// SessionManagerClaimsIssuer fills the "iss" field of the token.
	SessionManagerClaimsIssuer = "argocd"

	// invalidLoginError, for security purposes, doesn't say whether the username or password was invalid.  This does not mitigate the potential for timing attacks to determine which is which.
	invalidLoginError  = "Invalid username or password"
	blankPasswordError = "Blank passwords are not allowed"
	badUserError       = "Bad local superuser username"
)

// NewSessionManager creates a new session manager from Argo CD settings
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
func (mgr *SessionManager) Create(subject string, secondsBeforeExpiry int64) (string, error) {
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
		// Argo CD signed token
		return mgr.Parse(tokenString)
	default:
		// Dex signed token
		provider, err := mgr.oidcProvider()
		if err != nil {
			return nil, err
		}
		verifier := provider.Verifier(&oidc.Config{ClientID: claims.Audience})
		idToken, err := verifier.Verify(context.Background(), tokenString)
		if err != nil {
			// HACK: if we failed token verification, it's possible the reason was because dex
			// restarted and has new JWKS signing keys (we do not back dex with persistent storage
			// so keys might be regenerated). Detect this by:
			// 1. looking for the specific error message
			// 2. re-initializing the OIDC provider
			// 3. re-attempting token verification
			// NOTE: the error message is sensitive to implementation of verifier.Verify()
			if !strings.Contains(err.Error(), "failed to verify signature") {
				return nil, err
			}
			provider, retryErr := mgr.initializeOIDCProvider()
			if retryErr != nil {
				// return original error if we fail to re-initialize OIDC
				return nil, err
			}
			verifier = provider.Verifier(&oidc.Config{ClientID: claims.Audience})
			idToken, err = verifier.Verify(context.Background(), tokenString)
			if err != nil {
				return nil, err
			}
			// If we get here, we successfully re-initialized OIDC and after re-initialization,
			// the token is now valid.
			log.Info("New OIDC settings detected")
		}
		var claims jwt.MapClaims
		err = idToken.Claims(&claims)
		return claims, err
	}
}

// Username is a helper to extract a human readable username from a context
func Username(ctx context.Context) string {
	claims, ok := ctx.Value("claims").(jwt.Claims)
	if !ok {
		return ""
	}
	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		return ""
	}
	switch jwtutil.GetField(mapClaims, "iss") {
	case SessionManagerClaimsIssuer:
		return jwtutil.GetField(mapClaims, "sub")
	default:
		return jwtutil.GetField(mapClaims, "email")
	}
}

// oidcProvider lazily initializes, memoizes, and returns the OIDC provider.
// We have to initialize the provider lazily since Argo CD can be an OIDC client to itself (in the
// case of dex reverse proxy), which presents a chicken-and-egg problem of (1) serving dex over
// HTTP, and (2) querying the OIDC provider (ourself) to initialize the app.
func (mgr *SessionManager) oidcProvider() (*oidc.Provider, error) {
	if mgr.provider != nil {
		return mgr.provider, nil
	}
	return mgr.initializeOIDCProvider()
}

// initializeOIDCProvider re-initializes the OIDC provider, querying the well known oidc
// configuration path (http://example-argocd.com/api/dex/.well-known/openid-configuration)
func (mgr *SessionManager) initializeOIDCProvider() (*oidc.Provider, error) {
	if !mgr.settings.IsSSOConfigured() {
		return nil, fmt.Errorf("SSO is not configured")
	}
	provider, err := oidcutil.NewOIDCProvider(mgr.settings.IssuerURL(), mgr.client)
	if err != nil {
		return nil, err
	}
	mgr.provider = provider
	return mgr.provider, nil
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
