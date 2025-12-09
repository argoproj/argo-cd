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
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/dex"
	"github.com/argoproj/argo-cd/v3/util/env"
	httputil "github.com/argoproj/argo-cd/v3/util/http"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	oidcutil "github.com/argoproj/argo-cd/v3/util/oidc"
	passwordutil "github.com/argoproj/argo-cd/v3/util/password"
	"github.com/argoproj/argo-cd/v3/util/settings"
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
	failedLock                    sync.RWMutex
	metricsRegistry               MetricsRegistry
}

// LoginAttempts is a timestamped counter for failed login attempts
type LoginAttempts struct {
	// Time of the last failed login
	LastFailed time.Time `json:"lastFailed"`
	// Number of consecutive login failures
	FailCount int `json:"failCount"`
}

type MetricsRegistry interface {
	IncLoginRequestCounter(status string)
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
	defaultMaxCacheSize = 10000
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

var InvalidLoginErr = status.Errorf(codes.Unauthenticated, invalidLoginError)

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
func NewSessionManager(settingsMgr *settings.SettingsManager, projectsLister v1alpha1.AppProjectNamespaceLister, dexServerAddr string, dexTLSConfig *dex.DexTLSConfig, storage UserStateStorage) *SessionManager {
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

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	s.client = &http.Client{
		Transport: transport,
	}

	if settings.DexConfig != "" {
		transport.TLSClientConfig = dex.TLSConfig(dexTLSConfig)
		addrWithProto := dex.DexServerAddressWithProtocol(dexServerAddr, dexTLSConfig)
		s.client.Transport = dex.NewDexRewriteURLRoundTripper(addrWithProto, s.client.Transport)
	} else {
		transport.TLSClientConfig = settings.OIDCTLSConfig()
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

func (mgr *SessionManager) CollectMetrics(registry MetricsRegistry) {
	mgr.metricsRegistry = registry
	if mgr.metricsRegistry == nil {
		log.Warn("Metrics registry is not set, metrics will not be collected")
		return
	}
}

func (mgr *SessionManager) IncLoginRequestCounter(status string) {
	if mgr.metricsRegistry != nil {
		mgr.metricsRegistry.IncLoginRequestCounter(status)
	}
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
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
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

	subject := jwtutil.GetUserIdentifier(claims)
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
		return nil, "", errors.New("account password has changed since token issued")
	}

	newToken := ""
	exp, expErr := jwtutil.ExpirationTime(claims)
	iat, iatErr := jwtutil.IssuedAtTime(claims)

	// Only attempt auto-regeneration if we have both expiration and issuedAt times
	if expErr == nil && iatErr == nil && exp != nil && iat != nil {
		tokenExpDuration := exp.Sub(*iat)     // Dereference pointers
		remainingDuration := time.Until(*exp) // Dereference pointer

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

// GetLoginFailures retrieves the login failure information from the cache. Any modifications to the LoginAttemps map must be done in a thread-safe manner.
func (mgr *SessionManager) GetLoginFailures() map[string]LoginAttempts {
	// Get failures from the cache
	return mgr.storage.GetLoginAttempts()
}

func expireOldFailedAttempts(maxAge time.Duration, failures map[string]LoginAttempts) int {
	expiredCount := 0
	for key, attempt := range failures {
		if time.Since(attempt.LastFailed) > maxAge*time.Second {
			expiredCount++
			delete(failures, key)
		}
	}
	return expiredCount
}

// Protect admin user from login attempt reset caused by attempts to overflow cache in a brute force attack. Instead remove random non-admin to make room in cache.
func pickRandomNonAdminLoginFailure(failures map[string]LoginAttempts, username string) *string {
	idx := rand.Intn(len(failures) - 1)
	i := 0
	for key := range failures {
		if i == idx {
			if key == common.ArgoCDAdminUsername || key == username {
				return pickRandomNonAdminLoginFailure(failures, username)
			}
			return &key
		}
		i++
	}
	return nil
}

// Updates the failure count for a given username. If failed is true, increases the counter. Otherwise, sets counter back to 0.
func (mgr *SessionManager) updateFailureCount(username string, failed bool) {
	mgr.failedLock.Lock()
	defer mgr.failedLock.Unlock()

	failures := mgr.GetLoginFailures()

	// Expire old entries in the cache if we have a failure window defined.
	if window := getLoginFailureWindow(); window > 0 {
		count := expireOldFailedAttempts(window, failures)
		if count > 0 {
			log.Infof("Expired %d entries from session cache due to max age reached", count)
		}
	}

	// If we exceed a certain cache size, we need to remove random entries to
	// prevent overbloating the cache with fake entries, as this could lead to
	// memory exhaustion and ultimately in a DoS. We remove a single entry to
	// replace it with the new one.
	if failed && len(failures) >= getMaximumCacheSize() {
		log.Warnf("Session cache size exceeds %d entries, removing random entry", getMaximumCacheSize())
		rmUser := pickRandomNonAdminLoginFailure(failures, username)
		if rmUser != nil {
			delete(failures, *rmUser)
			log.Infof("Deleted entry for user %s from cache", *rmUser)
		}
	}

	attempt, ok := failures[username]
	if !ok {
		attempt = LoginAttempts{FailCount: 0}
	}

	// On login failure, increase fail count and update last failed timestamp.
	// On login success, remove the entry from the cache.
	if failed {
		attempt.FailCount++
		attempt.LastFailed = time.Now()
		failures[username] = attempt
		log.Warnf("User %s failed login %d time(s)", username, attempt.FailCount)
	} else if attempt.FailCount > 0 {
		// Forget username for cache size enforcement, since entry in cache was deleted
		delete(failures, username)
	}

	err := mgr.storage.SetLoginAttempts(failures)
	if err != nil {
		log.Errorf("Could not update login attempts: %v", err)
	}
}

// Get the current login failure attempts for given username
func (mgr *SessionManager) getFailureCount(username string) LoginAttempts {
	mgr.failedLock.RLock()
	defer mgr.failedLock.RUnlock()
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

// AuthMiddlewareFunc returns a function that can be used as an
// authentication middleware for HTTP requests.
func (mgr *SessionManager) AuthMiddlewareFunc(disabled bool, isSSOConfigured bool, ssoClientApp *oidcutil.ClientApp) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return WithAuthMiddleware(disabled, isSSOConfigured, ssoClientApp, mgr, h)
	}
}

// TokenVerifier defines the contract to invoke token
// verification logic
type TokenVerifier interface {
	VerifyToken(ctx context.Context, token string) (jwt.Claims, string, error)
}

// WithAuthMiddleware is an HTTP middleware used to ensure incoming requests are authenticated before invoking the target handler.
// If disabled is true, it will just invoke the next handler in the chain.
// It checks for tokens in a configured header (for IAP JWTs) first, then falls back to cookies.
func WithAuthMiddleware(disabled bool, isSSOConfigured bool, ssoClientApp *oidcutil.ClientApp, authn TokenVerifier, next http.Handler) http.Handler {
	if disabled {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string
		var err error
		ctx := r.Context()

		// Attempt to get settings manager from the verifier if possible
		// This assumes the TokenVerifier implementation might expose settings
		var settingsMgr *settings.SettingsManager
		if sm, ok := authn.(*SessionManager); ok {
			settingsMgr = sm.settingsMgr
		}

		// 1. Check Header for JWT (if configured)
		if settingsMgr != nil {
			argoSettings, settingsErr := settingsMgr.GetSettings()
			if settingsErr == nil && argoSettings.IsJWTConfigured() {
				tokenString = r.Header.Get(argoSettings.JWTConfig.HeaderName)
				if tokenString != "" {
					log.Debugf("Found token in header %s", argoSettings.JWTConfig.HeaderName)
				}
			} else if settingsErr != nil {
				log.Warnf("Failed to get settings for JWT header check: %v", settingsErr)
			}
		}

		// 2. Fallback to Cookie
		if tokenString == "" {
			cookies := r.Cookies()
			tokenString, err = httputil.JoinCookies(common.AuthCookieName, cookies)
			if err != nil {
				http.Error(w, "Auth cookie not found", http.StatusBadRequest)
				return
			}
			log.Debug("Found token in cookie")
		}

		// 3. Verify Token
		claims, _, err := authn.VerifyToken(ctx, tokenString)
		if err != nil {
			log.Warnf("Token verification failed: %v", err)
			// Add error to context for potential downstream handling (e.g., UI showing expired message)
			//nolint:staticcheck
			r = r.WithContext(context.WithValue(ctx, AuthErrorCtxKey, err))
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		finalClaims := claims
		if isSSOConfigured {
			finalClaims, err = ssoClientApp.SetGroupsFromUserInfo(ctx, claims, SessionManagerClaimsIssuer)
			if err != nil {
				http.Error(w, "Invalid session", http.StatusUnauthorized)
				return
			}
		}

		// 4. Add claims to context
		//nolint:staticcheck
		ctx = context.WithValue(ctx, "claims", finalClaims)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// VerifyToken verifies if a token is correct. Tokens can be issued either from us, an IDP, or an IAP.
// We choose how to verify based on settings and the token issuer.
func (mgr *SessionManager) VerifyToken(ctx context.Context, tokenString string) (jwt.Claims, string, error) {
	argoSettings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return nil, "", fmt.Errorf("cannot access settings while verifying the token: %w", err)
	}
	if argoSettings == nil {
		return nil, "", errors.New("settings are not available while verifying the token")
	}

	// 1. Attempt JWT verification (for IAP) if configured
	if argoSettings.IsJWTConfigured() {
		prov, err := mgr.provider() // provider() handles lazy init
		if err != nil {
			// Log the error but don't fail immediately, maybe it's an Argo CD token
			log.Warnf("Failed to get OIDC provider for JWT verification: %v", err)
		} else {
			token, jwtErr := prov.VerifyJWT(ctx, tokenString, argoSettings)
			if jwtErr == nil {
				// Successfully verified as JWT via JWKS URL
				log.Debugf("Token verified using JWT config (JWKS URL), claims: %v", token.Claims)
				return token.Claims, "", nil
			}
			// Log the JWT verification failure and continue to other methods
			log.Debugf("JWT verification failed, trying other methods: %v", jwtErr)
		}
	}

	// 2. Parse token unverified to check issuer for Argo CD or OIDC flow
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.MapClaims{}
	_, _, parseErr := parser.ParseUnverified(tokenString, &claims)
	if parseErr != nil {
		// If we couldn't even parse it, and JWT verification didn't work (or wasn't configured), fail.
		return nil, "", fmt.Errorf("failed to parse token: %w", parseErr)
	}

	// 3. Check if it's an Argo CD issued token
	issuer, issErr := claims.GetIssuer()
	if issErr != nil {
		log.Debugf("Could not get issuer claim, assuming not Argo CD token: %v", issErr)
	} else if issuer == SessionManagerClaimsIssuer {
		log.Debug("Attempting verification as Argo CD token")
		return mgr.Parse(tokenString)
	}

	// 4. Attempt OIDC verification (external IDP or Dex)
	log.Debugf("Attempting verification as OIDC token (issuer: %s)", issuer) // Use issuer variable from above
	prov, err := mgr.provider()
	if err != nil {
		// If OIDC/Dex is not configured, but we reached here, the token is invalid
		return nil, "", fmt.Errorf("token issuer (%s) is not '%s' and OIDC provider is not available: %w", issuer, SessionManagerClaimsIssuer, err)
	}

	idToken, err := prov.Verify(ctx, tokenString, argoSettings)
	if err != nil {
		log.Warnf("OIDC token verification failed: %s", err)
		// Handle expired token specifically for UI hints using errors.As
		var tokenExpiredError *oidc.TokenExpiredError
		if errors.As(err, &tokenExpiredError) {
			// Return minimal claims indicating SSO source for expired tokens
			// Use issuer variable from above, handle potential error if it wasn't retrieved
			issForExpired := ""
			if issErr == nil {
				issForExpired = issuer
			}
			expiredClaims := jwt.MapClaims{"iss": issForExpired}
			log.Debugf("OIDC token expired: %v", err)
			return expiredClaims, "", common.ErrTokenVerification // Return specific error? Maybe jwt.ErrTokenExpired?
		}
		// Check for other specific OIDC errors if needed
		// ...
		return nil, "", common.ErrTokenVerification // Return generic verification error
	}

	// Successfully verified via OIDC
	var verifiedClaims jwt.MapClaims
	if claimsErr := idToken.Claims(&verifiedClaims); claimsErr != nil {
		return nil, "", fmt.Errorf("failed to extract claims from verified OIDC token: %w", claimsErr)
	}
	log.Debug("Token verified using OIDC config")
	return verifiedClaims, "", nil
}

func (mgr *SessionManager) provider() (oidcutil.Provider, error) {
	if mgr.prov != nil {
		return mgr.prov, nil
	}
	settings, err := mgr.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}
	// In the case of external JWT we need an OIDC provider to veryify tokens
	if !settings.IsSSOConfigured() && !settings.IsJWTConfigured() {
		return nil, errors.New("SSO or JWT is not configured")
	}
	mgr.prov = oidcutil.NewOIDCProvider(settings.IssuerURL(), mgr.client)
	return mgr.prov, nil
}

func (mgr *SessionManager) RevokeToken(ctx context.Context, id string, expiringAt time.Duration) error {
	return mgr.storage.RevokeToken(ctx, id, expiringAt)
}

func LoggedIn(ctx context.Context) bool {
	return GetUserIdentifier(ctx) != "" && ctx.Value(AuthErrorCtxKey) == nil
}

// Username is a helper to extract a human readable username from a context
func Username(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	switch jwtutil.StringField(mapClaims, "iss") {
	case SessionManagerClaimsIssuer:
		return jwtutil.GetUserIdentifier(mapClaims)
	default:
		e := jwtutil.StringField(mapClaims, "email")
		if e != "" {
			return e
		}
		return jwtutil.GetUserIdentifier(mapClaims)
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
	t, err := jwtutil.IssuedAtTime(mapClaims)
	if err != nil || t == nil {
		// Return zero time and the error if extraction failed or result is nil
		return time.Time{}, err
	}
	return *t, nil // Dereference the pointer
}

// GetUserIdentifier returns the user identifier from context, prioritizing federated claims over subject
func GetUserIdentifier(ctx context.Context) string {
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return ""
	}
	return jwtutil.GetUserIdentifier(mapClaims)
}

func Groups(ctx context.Context, scopes []string) []string { // Added settingsMgr parameter
	mapClaims, ok := mapClaims(ctx)
	if !ok {
		return nil
	}
	// Group extraction, includes JWT groups if set
	groups := jwtutil.GetGroups(mapClaims, scopes)
	log.Printf("Extracted groups from token claims: %v", groups)
	return groups
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
