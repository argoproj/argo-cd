package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/argoproj/argo-cd/server/rbacpolicy"

	"github.com/dgrijalva/jwt-go"
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
	accountDisabled    = "Account %s is disabled"
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
// The id parameter holds an optional unique JWT token identifier and stored as a standard claim "jti" in the JWT token.
func (mgr *SessionManager) Create(subject string, secondsBeforeExpiry int64, id string) (string, error) {
	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	now := time.Now().UTC()
	claims := jwt.StandardClaims{
		IssuedAt:  now.Unix(),
		Issuer:    SessionManagerClaimsIssuer,
		NotBefore: now.Unix(),
		Subject:   subject,
		Id:        id,
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

// Parse tries to parse the provided string and returns the token claims for local login.
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

	subject := jwtutil.GetField(claims, "sub")
	if rbacpolicy.IsProjectSubject(subject) {
		return token.Claims, nil
	}

	account, err := mgr.settingsMgr.GetAccount(subject)
	if err != nil {
		return nil, err
	}

	if id := jwtutil.GetField(claims, "jti"); id != "" && account.TokenIndex(id) == -1 {
		return nil, fmt.Errorf("account %s does not have token with id %s", subject, id)
	}

	issuedAt := time.Unix(int64(claims["iat"].(float64)), 0)
	if account.PasswordMtime != nil && issuedAt.Before(*account.PasswordMtime) {
		return nil, fmt.Errorf("Account password has changed since token issued")
	}
	return token.Claims, nil
}

// VerifyUsernamePassword verifies if a username/password combo is correct
func (mgr *SessionManager) VerifyUsernamePassword(username string, password string) error {
	account, err := mgr.settingsMgr.GetAccount(username)
	if err != nil {
		return err
	}
	if !account.Enabled {
		return status.Errorf(codes.Unauthenticated, accountDisabled, username)
	}
	if password == "" {
		return status.Errorf(codes.Unauthenticated, blankPasswordError)
	}

	valid, _ := passwordutil.VerifyPassword(password, account.PasswordHash)
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
			return claims, err
		}
		idToken, err := prov.Verify(claims.Audience, tokenString)
		if err != nil {
			return claims, err
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

func LoggedIn(ctx context.Context) bool {
	return Sub(ctx) != ""
}

// Username is a helper to extract a human readable username from a context
func Username(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	switch jwtutil.GetField(mapClaims, "iss") {
	case SessionManagerClaimsIssuer:
		return jwtutil.GetField(mapClaims, "sub")
	default:
		return jwtutil.GetField(mapClaims, "email")
	}
}

func Iss(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	return jwtutil.GetField(mapClaims, "iss")
}

func Iat(ctx context.Context) (time.Time, error) {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return time.Time{}, errors.New("unable to extract token claims")
	}
	iatField, ok := mapClaims["iat"]
	if !ok {
		return time.Time{}, errors.New("token does not have iat claim")
	}

	if iat, ok := iatField.(float64); !ok {
		return time.Time{}, errors.New("iat token field has unexpected type")
	} else {
		return time.Unix(int64(iat), 0), nil
	}
}

func Sub(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	return jwtutil.GetField(mapClaims, "sub")
}

func Groups(ctx context.Context) []string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return nil
	}
	return jwtutil.GetGroups(mapClaims)
}

func mapClaims(ctx context.Context) (jwt.MapClaims, bool) {
	claims, ok := ctx.Value("claims").(jwt.Claims)
	if !ok {
		return nil, false
	}
	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		return nil, false
	}
	return mapClaims, true
}
