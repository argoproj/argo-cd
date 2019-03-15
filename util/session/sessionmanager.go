package session

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/dex"
	httputil "github.com/argoproj/argo-cd/util/http"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	oidcutil "github.com/argoproj/argo-cd/util/oidc"
	passwordutil "github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/settings"
)

// SessionManager generates and validates JWT tokens for login sessions.
type SessionManager struct {
	settingsMgr *settings.SettingsManager
	client      *http.Client
	prov        oidcutil.Provider
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
func NewSessionManager(settingsMgr *settings.SettingsManager, dexServerAddr string) *SessionManager {
	s := SessionManager{
		settingsMgr: settingsMgr,
	}
	settings, err := settingsMgr.GetSettings()
	if err != nil {
		panic(err)
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
	if settings.DexConfig != "" {
		s.client.Transport = dex.NewDexRewriteURLRoundTripper(dexServerAddr, s.client.Transport)
	}
	if os.Getenv(common.EnvVarSSODebug) == "1" {
		s.client.Transport = httputil.DebugTransport{T: s.client.Transport}
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
	settings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return "", err
	}
	return token.SignedString(settings.ServerSignature)
}

// Parse tries to parse the provided string and returns the token claims for local superuser login.
func (mgr *SessionManager) Parse(tokenString string) (jwt.Claims, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	var claims jwt.MapClaims
	settings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return settings.ServerSignature, nil
	})
	if err != nil {
		return nil, err
	}

	issuedAt := time.Unix(int64(claims["iat"].(float64)), 0)
	if issuedAt.Before(settings.AdminPasswordMtime) {
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
	settings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return err
	}
	valid, _ := passwordutil.VerifyPassword(password, settings.AdminPasswordHash)
	if !valid {
		return status.Errorf(codes.Unauthenticated, invalidLoginError)
	}
	return nil
}

// VerifyToken verifies if a token is correct. Tokens can be issued either from us or by an IDP.
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
		// IDP signed token
		prov, err := mgr.provider()
		if err != nil {
			return nil, err
		}
		idToken, err := prov.Verify(claims.Audience, tokenString)
		if err != nil {
			return nil, err
		}
		var claims jwt.MapClaims
		err = idToken.Claims(&claims)
		return claims, err
	}
}

func (mgr *SessionManager) provider() (oidcutil.Provider, error) {
	if mgr.prov != nil {
		return mgr.prov, nil
	}
	settings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}
	if !settings.IsSSOConfigured() {
		return nil, fmt.Errorf("SSO is not configured")
	}
	mgr.prov = oidcutil.NewOIDCProvider(settings.IssuerURL(), mgr.client)
	return mgr.prov, nil
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
