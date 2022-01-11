package session

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/dex"
	"github.com/argoproj/argo-cd/v2/util/env"
	httputil "github.com/argoproj/argo-cd/v2/util/http"
	jwtutil "github.com/argoproj/argo-cd/v2/util/jwt"
	oidcutil "github.com/argoproj/argo-cd/v2/util/oidc"
	passwordutil "github.com/argoproj/argo-cd/v2/util/password"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

// SessionManager generates and validates JWT tokens for login sessions.
type SessionManager struct {
	settingsMgr                   *settings.SettingsManager
	projectsLister                v1alpha1.AppProjectNamespaceLister
	client                        *http.Client
	prov                          oidcutil.Provider
	storage                       UserStateStorage
	sleep                         func(d time.Duration)
	verificationDelayNoiseEnabled bool
}

// LoginAttempts is a timestamped counter for failed login attempts
type LoginAttempts struct {
	// Time of the last failed login
	LastFailed time.Time `json:"lastFailed"`
	// Number of consecutive login failures
	FailCount int `json:"failCount"`
}

const (
	// SessionManagerClaimsIssuer fills the "iss" field of the token.
	SessionManagerClaimsIssuer = "argocd"
	AuthErrorCtxKey            = "auth-error"

	// invalidLoginError, for security purposes, doesn't say whether the username or password was invalid.  This does not mitigate the potential for timing attacks to determine which is which.
	invalidLoginError           = "Invalid username or password"
	blankPasswordError          = "Blank passwords are not allowed"
	accountDisabled             = "Account %s is disabled"
	usernameTooLongError        = "Username is too long (%d bytes max)"
	userDoesNotHaveCapability   = "Account %s does not have %s capability"
	autoRegenerateTokenDuration = time.Minute * 5
)

const (
	// Maximum length of username, too keep the cache's memory signature low
	maxUsernameLength = 32
	// The default maximum session cache size
	defaultMaxCacheSize = 1000
	// The default number of maximum login failures before delay kicks in
	defaultMaxLoginFailures = 5
	// The default time in seconds for the failure window
	defaultFailureWindow = 300
	// The password verification delay max
	verificationDelayNoiseMin = 500 * time.Millisecond
	// The password verification delay max
	verificationDelayNoiseMax = 1000 * time.Millisecond

	// environment variables to control rate limiter behaviour:

	// Max number of login failures before login delay kicks in
	envLoginMaxFailCount = "ARGOCD_SESSION_FAILURE_MAX_FAIL_COUNT"

	// Number of maximum seconds the login is allowed to delay for. Default: 300 (5 minutes).
	envLoginFailureWindowSeconds = "ARGOCD_SESSION_FAILURE_WINDOW_SECONDS"

	// Max number of stored usernames
	envLoginMaxCacheSize = "ARGOCD_SESSION_MAX_CACHE_SIZE"
)

var (
	InvalidLoginErr = status.Errorf(codes.Unauthenticated, invalidLoginError)
)

// Returns the maximum cache size as number of entries
func getMaximumCacheSize() int {
	return env.ParseNumFromEnv(envLoginMaxCacheSize, defaultMaxCacheSize, 1, math.MaxInt32)
}

// Returns the maximum number of login failures before login delay kicks in
func getMaxLoginFailures() int {
	return env.ParseNumFromEnv(envLoginMaxFailCount, defaultMaxLoginFailures, 1, math.MaxInt32)
}

// Returns the number of maximum seconds the login is allowed to delay for
func getLoginFailureWindow() time.Duration {
	return time.Duration(env.ParseNumFromEnv(envLoginFailureWindowSeconds, defaultFailureWindow, 0, math.MaxInt32))
}

// NewSessionManager creates a new session manager from Argo CD settings
func NewSessionManager(settingsMgr *settings.SettingsManager, projectsLister v1alpha1.AppProjectNamespaceLister, dexServerAddr string, storage UserStateStorage) *SessionManager {
	s := SessionManager{
		settingsMgr:                   settingsMgr,
		storage:                       storage,
		sleep:                         time.Sleep,
		projectsLister:                projectsLister,
		verificationDelayNoiseEnabled: true,
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
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    SessionManagerClaimsIssuer,
		NotBefore: jwt.NewNumericDate(now),
		Subject:   subject,
		ID:        id,
	}
	if secondsBeforeExpiry > 0 {
		expires := now.Add(time.Duration(secondsBeforeExpiry) * time.Second)
		claims.ExpiresAt = jwt.NewNumericDate(expires)
	}

	return mgr.signClaims(claims)
}

func (mgr *SessionManager) signClaims(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	settings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return "", err
	}
	return token.SignedString(settings.ServerSignature)
}

// GetSubjectAccountAndCapability analyzes Argo CD account token subject and extract account name
// and the capability it was generated for (default capability is API Key).
func GetSubjectAccountAndCapability(subject string) (string, settings.AccountCapability) {
	capability := settings.AccountCapabilityApiKey
	if parts := strings.Split(subject, ":"); len(parts) > 1 {
		subject = parts[0]
		switch parts[1] {
		case string(settings.AccountCapabilityLogin):
			capability = settings.AccountCapabilityLogin
		case string(settings.AccountCapabilityApiKey):
			capability = settings.AccountCapabilityApiKey
		}
	}
	return subject, capability
}

// Parse tries to parse the provided string and returns the token claims for local login.
func (mgr *SessionManager) Parse(tokenString string) (jwt.Claims, string, error) {
	// Parse takes the token string and a function for looking up the key. The latter is especially
	// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
	// head of the token to identify which key to use, but the parsed token (head and claims) is provided
	// to the callback, providing flexibility.
	var claims jwt.MapClaims
	argoCDSettings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return nil, "", err
	}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return argoCDSettings.ServerSignature, nil
	})
	if err != nil {
		return nil, "", err
	}

	issuedAt, err := jwtutil.IssuedAtTime(claims)
	if err != nil {
		return nil, "", err
	}

	subject := jwtutil.StringField(claims, "sub")
	id := jwtutil.StringField(claims, "jti")

	if projName, role, ok := rbacpolicy.GetProjectRoleFromSubject(subject); ok {
		proj, err := mgr.projectsLister.Get(projName)
		if err != nil {
			return nil, "", err
		}
		_, _, err = proj.GetJWTToken(role, issuedAt.Unix(), id)
		if err != nil {
			return nil, "", err
		}

		return token.Claims, "", nil
	}

	subject, capability := GetSubjectAccountAndCapability(subject)
	claims["sub"] = subject

	account, err := mgr.settingsMgr.GetAccount(subject)
	if err != nil {
		return nil, "", err
	}

	if !account.Enabled {
		return nil, "", fmt.Errorf("account %s is disabled", subject)
	}

	if !account.HasCapability(capability) {
		return nil, "", fmt.Errorf("account %s does not have '%s' capability", subject, capability)
	}

	if id == "" || mgr.storage.IsTokenRevoked(id) {
		return nil, "", errors.New("token is revoked, please re-login")
	} else if capability == settings.AccountCapabilityApiKey && account.TokenIndex(id) == -1 {
		return nil, "", fmt.Errorf("account %s does not have token with id %s", subject, id)
	}

	if account.PasswordMtime != nil && issuedAt.Before(*account.PasswordMtime) {
		return nil, "", fmt.Errorf("Account password has changed since token issued")
	}

	newToken := ""
	if exp, err := jwtutil.ExpirationTime(claims); err == nil {
		tokenExpDuration := exp.Sub(issuedAt)
		remainingDuration := time.Until(exp)

		if remainingDuration < autoRegenerateTokenDuration && capability == settings.AccountCapabilityLogin {
			if uniqueId, err := uuid.NewRandom(); err == nil {
				if val, err := mgr.Create(fmt.Sprintf("%s:%s", subject, settings.AccountCapabilityLogin), int64(tokenExpDuration.Seconds()), uniqueId.String()); err == nil {
					newToken = val
				}
			}
		}
	}
	return token.Claims, newToken, nil
}

// GetLoginFailures retrieves the login failure information from the cache
func (mgr *SessionManager) GetLoginFailures() map[string]LoginAttempts {
	// Get failures from the cache
	var failures map[string]LoginAttempts
	err := mgr.storage.GetLoginAttempts(&failures)
	if err != nil {
		if err != appstate.ErrCacheMiss {
			log.Errorf("Could not retrieve login attempts: %v", err)
		}
		failures = make(map[string]LoginAttempts)
	}

	return failures
}

func expireOldFailedAttempts(maxAge time.Duration, failures *map[string]LoginAttempts) int {
	expiredCount := 0
	for key, attempt := range *failures {
		if time.Since(attempt.LastFailed) > maxAge*time.Second {
			expiredCount += 1
			delete(*failures, key)
		}
	}
	return expiredCount
}

// Updates the failure count for a given username. If failed is true, increases the counter. Otherwise, sets counter back to 0.
func (mgr *SessionManager) updateFailureCount(username string, failed bool) {

	failures := mgr.GetLoginFailures()

	// Expire old entries in the cache if we have a failure window defined.
	if window := getLoginFailureWindow(); window > 0 {
		count := expireOldFailedAttempts(window, &failures)
		if count > 0 {
			log.Infof("Expired %d entries from session cache due to max age reached", count)
		}
	}

	// If we exceed a certain cache size, we need to remove random entries to
	// prevent overbloating the cache with fake entries, as this could lead to
	// memory exhaustion and ultimately in a DoS. We remove a single entry to
	// replace it with the new one.
	//
	// Chances are that we remove the one that is under active attack, but this
	// chance is low (1:cache_size)
	if failed && len(failures) >= getMaximumCacheSize() {
		log.Warnf("Session cache size exceeds %d entries, removing random entry", getMaximumCacheSize())
		idx := rand.Intn(len(failures) - 1)
		var rmUser string
		i := 0
		for key := range failures {
			if i == idx {
				rmUser = key
				delete(failures, key)
				break
			}
			i++
		}
		log.Infof("Deleted entry for user %s from cache", rmUser)
	}

	attempt, ok := failures[username]
	if !ok {
		attempt = LoginAttempts{FailCount: 0}
	}

	// On login failure, increase fail count and update last failed timestamp.
	// On login success, remove the entry from the cache.
	if failed {
		attempt.FailCount += 1
		attempt.LastFailed = time.Now()
		failures[username] = attempt
		log.Warnf("User %s failed login %d time(s)", username, attempt.FailCount)
	} else {
		if attempt.FailCount > 0 {
			// Forget username for cache size enforcement, since entry in cache was deleted
			delete(failures, username)
		}
	}

	err := mgr.storage.SetLoginAttempts(failures)
	if err != nil {
		log.Errorf("Could not update login attempts: %v", err)
	}

}

// Get the current login failure attempts for given username
func (mgr *SessionManager) getFailureCount(username string) LoginAttempts {
	failures := mgr.GetLoginFailures()
	attempt, ok := failures[username]
	if !ok {
		attempt = LoginAttempts{FailCount: 0}
	}
	return attempt
}

// Calculate a login delay for the given login attempt
func (mgr *SessionManager) exceededFailedLoginAttempts(attempt LoginAttempts) bool {
	maxFails := getMaxLoginFailures()
	failureWindow := getLoginFailureWindow()

	// Whether we are in the failure window for given attempt
	inWindow := func() bool {
		if failureWindow == 0 || time.Since(attempt.LastFailed).Seconds() <= float64(failureWindow) {
			return true
		}
		return false
	}

	// If we reached max failed attempts within failure window, we need to calc the delay
	if attempt.FailCount >= maxFails && inWindow() {
		return true
	}

	return false
}

// VerifyUsernamePassword verifies if a username/password combo is correct
func (mgr *SessionManager) VerifyUsernamePassword(username string, password string) error {
	if password == "" {
		return status.Errorf(codes.Unauthenticated, blankPasswordError)
	}
	// Enforce maximum length of username on local accounts
	if len(username) > maxUsernameLength {
		return status.Errorf(codes.InvalidArgument, usernameTooLongError, maxUsernameLength)
	}

	start := time.Now()
	if mgr.verificationDelayNoiseEnabled {
		defer func() {
			// introduces random delay to protect from timing-based user enumeration attack
			delayNanoseconds := verificationDelayNoiseMin.Nanoseconds() +
				int64(rand.Intn(int(verificationDelayNoiseMax.Nanoseconds()-verificationDelayNoiseMin.Nanoseconds())))
				// take into account amount of time spent since the request start
			delayNanoseconds = delayNanoseconds - time.Since(start).Nanoseconds()
			if delayNanoseconds > 0 {
				mgr.sleep(time.Duration(delayNanoseconds))
			}
		}()
	}

	attempt := mgr.getFailureCount(username)
	if mgr.exceededFailedLoginAttempts(attempt) {
		log.Warnf("User %s had too many failed logins (%d)", username, attempt.FailCount)
		return InvalidLoginErr
	}

	account, err := mgr.settingsMgr.GetAccount(username)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
			mgr.updateFailureCount(username, true)
			err = InvalidLoginErr
		}
		// to prevent time-based user enumeration, we must perform a password
		// hash cycle to keep response time consistent (if the function were
		// to continue and not return here)
		_, _ = passwordutil.HashPassword("for_consistent_response_time")
		return err
	}

	valid, _ := passwordutil.VerifyPassword(password, account.PasswordHash)
	if !valid {
		mgr.updateFailureCount(username, true)
		return InvalidLoginErr
	}

	if !account.Enabled {
		return status.Errorf(codes.Unauthenticated, accountDisabled, username)
	}

	if !account.HasCapability(settings.AccountCapabilityLogin) {
		return status.Errorf(codes.Unauthenticated, userDoesNotHaveCapability, username, settings.AccountCapabilityLogin)
	}
	mgr.updateFailureCount(username, false)
	return nil
}

// VerifyToken verifies if a token is correct. Tokens can be issued either from us or by an IDP.
// We choose how to verify based on the issuer.
func (mgr *SessionManager) VerifyToken(tokenString string) (jwt.Claims, string, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	var claims jwt.RegisteredClaims
	_, _, err := parser.ParseUnverified(tokenString, &claims)
	if err != nil {
		return nil, "", err
	}
	switch claims.Issuer {
	case SessionManagerClaimsIssuer:
		// Argo CD signed token
		return mgr.Parse(tokenString)
	default:
		// IDP signed token
		prov, err := mgr.provider()
		if err != nil {
			return claims, "", err
		}

		// Token must be verified for at least one audience
		// TODO(jannfis): Is this the right way? Shouldn't we know our audience and only validate for the correct one?
		var idToken *oidc.IDToken
		for _, aud := range claims.Audience {
			idToken, err = prov.Verify(aud, tokenString)
			if err == nil {
				break
			}
		}
		if err != nil {
			return claims, "", err
		}
		if idToken == nil {
			return claims, "", fmt.Errorf("No audience found in the token")
		}

		var claims jwt.MapClaims
		err = idToken.Claims(&claims)
		return claims, "", err
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

func (mgr *SessionManager) RevokeToken(ctx context.Context, id string, expiringAt time.Duration) error {
	return mgr.storage.RevokeToken(ctx, id, expiringAt)
}

func LoggedIn(ctx context.Context) bool {
	return Sub(ctx) != "" && ctx.Value(AuthErrorCtxKey) == nil
}

// Username is a helper to extract a human readable username from a context
func Username(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	switch jwtutil.StringField(mapClaims, "iss") {
	case SessionManagerClaimsIssuer:
		return jwtutil.StringField(mapClaims, "sub")
	default:
		return jwtutil.StringField(mapClaims, "email")
	}
}

func Iss(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	return jwtutil.StringField(mapClaims, "iss")
}

func Iat(ctx context.Context) (time.Time, error) {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return time.Time{}, errors.New("unable to extract token claims")
	}
	return jwtutil.IssuedAtTime(mapClaims)
}

func Sub(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	return jwtutil.StringField(mapClaims, "sub")
}

func Groups(ctx context.Context, scopes []string) []string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return nil
	}
	return jwtutil.GetGroups(mapClaims, scopes)
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
