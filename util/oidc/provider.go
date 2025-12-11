package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	jwtgo "github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Provider is a wrapper around go-oidc provider to also provide the following features:
// 1. lazy initialization/querying of the provider
// 2. automatic detection of change in signing keys
// 3. convenience function for verifying tokens
// We have to initialize the provider lazily since Argo CD can be an OIDC client to itself (in the
// case of dex reverse proxy), which presents a chicken-and-egg problem of (1) serving dex over
// HTTP, and (2) querying the OIDC provider (ourself) to initialize the OIDC client.
type Provider interface {
	Endpoint() (*oauth2.Endpoint, error)

	ParseConfig() (*OIDCConfiguration, error)

	Verify(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (*gooidc.IDToken, error)

	VerifyJWT(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (*jwtgo.Token, error)
}

type providerImpl struct {
	issuerURL      string
	client         *http.Client
	goOIDCProvider *gooidc.Provider

	jwksCache       *jose.JSONWebKeySet
	jwksExpiry      time.Time
	jwksCacheMux    sync.Mutex
	defaultCacheTTL time.Duration
}

var _ Provider = &providerImpl{}

// NewOIDCProvider initializes an OIDC provider
func NewOIDCProvider(issuerURL string, client *http.Client) Provider {
	return &providerImpl{
		issuerURL:       issuerURL,
		client:          client,
		defaultCacheTTL: 5 * time.Minute,
	}
}

// oidcProvider lazily initializes, memoizes, and returns the OIDC provider.
func (p *providerImpl) provider() (*gooidc.Provider, error) {
	if p.goOIDCProvider != nil {
		return p.goOIDCProvider, nil
	}
	prov, err := p.newGoOIDCProvider()
	if err != nil {
		return nil, err
	}
	p.goOIDCProvider = prov
	return p.goOIDCProvider, nil
}

// newGoOIDCProvider creates a new instance of go-oidc.Provider querying the well known oidc
// configuration path (http://example-argocd.com/api/dex/.well-known/openid-configuration)
func (p *providerImpl) newGoOIDCProvider() (*gooidc.Provider, error) {
	log.Infof("Initializing OIDC provider (issuer: %s)", p.issuerURL)
	ctx := gooidc.ClientContext(context.Background(), p.client)
	prov, err := gooidc.NewProvider(ctx, p.issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query provider %q: %w", p.issuerURL, err)
	}
	s, _ := ParseConfig(prov)
	log.Infof("OIDC supported scopes: %v", s.ScopesSupported)
	return prov, nil
}

type tokenVerificationError struct {
	errorsByAudience map[string]error
}

func (t tokenVerificationError) Error() string {
	var errorStrings []string
	for aud, err := range t.errorsByAudience {
		errorStrings = append(errorStrings, fmt.Sprintf("error for aud %q: %v", aud, err))
	}
	return "token verification failed for all audiences: " + strings.Join(errorStrings, ", ")
}

func (p *providerImpl) Verify(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (*gooidc.IDToken, error) {
	// According to the JWT spec, the aud claim is optional. The spec also says (emphasis mine):
	//
	//   If the principal processing the claim does not identify itself with a value in the "aud" claim _when this
	//   claim is present_, then the JWT MUST be rejected.
	//
	//     - https://www.rfc-editor.org/rfc/rfc7519#section-4.1.3
	//
	// If the claim is not present, we can skip the audience claim check (called the "ClientID check" in go-oidc's
	// terminology).
	//
	// The OIDC spec says that the aud claim is required (https://openid.net/specs/openid-connect-core-1_0.html#IDToken).
	// But we cannot assume that all OIDC providers will follow the spec. For Argo CD <2.6.0, we will default to
	// allowing the aud claim to be optional. In Argo CD >=2.6.0, we will default to requiring the aud claim to be
	// present and give users the skipAudienceCheckWhenTokenHasNoAudience setting to revert the behavior if necessary.
	//
	// At this point, we have not verified that the token has not been altered. All code paths below MUST VERIFY
	// THE TOKEN SIGNATURE to confirm that an attacker did not maliciously remove the "aud" claim.
	unverifiedHasAudClaim, err := security.UnverifiedHasAudClaim(tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether the token has an aud claim: %w", err)
	}

	var idToken *gooidc.IDToken
	if !unverifiedHasAudClaim {
		idToken, err = p.verify(ctx, "", tokenString, argoSettings.SkipAudienceCheckWhenTokenHasNoAudience())
	} else {
		allowedAudiences := argoSettings.OAuth2AllowedAudiences()
		if len(allowedAudiences) == 0 {
			return nil, errors.New("token has an audience claim, but no allowed audiences are configured")
		}
		tokenVerificationErrors := make(map[string]error)
		// Token must be verified for at least one allowed audience
		for _, aud := range allowedAudiences {
			idToken, err = p.verify(ctx, aud, tokenString, false)
			tokenExpiredError := &gooidc.TokenExpiredError{}
			if errors.As(err, &tokenExpiredError) {
				// If the token is expired, we won't bother checking other audiences. It's important to return a
				// TokenExpiredError instead of an error related to an incorrect audience, because the caller may
				// have specific behavior to handle expired tokens.
				break
			}
			if err == nil {
				break
			}
			// We store the error for each audience so that we can return a more detailed error message to the user.
			// If this gets merged, we'll be able to detect failures unrelated to audiences and short-circuit this loop
			// to avoid logging irrelevant warnings: https://github.com/coreos/go-oidc/pull/406
			tokenVerificationErrors[aud] = err
		}
		// If the most recent attempt encountered an error, and if we have collected multiple errors, switch to the
		// other error type to gather more context.
		if err != nil && len(tokenVerificationErrors) > 0 {
			err = tokenVerificationError{errorsByAudience: tokenVerificationErrors}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to verify provider token: %w", err)
	}

	return idToken, nil
}

// VerifyJWT verifies a JWT token using the configured JWK Set URL
func (p *providerImpl) VerifyJWT(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (*jwtgo.Token, error) {
	if !argoSettings.IsJWTConfigured() {
		return nil, errors.New("valid JWT configuration not found")
	}

	cacheTTL := p.defaultCacheTTL
	if argoSettings.JWTConfig.CacheTTL != "" {
		ttl, err := time.ParseDuration(argoSettings.JWTConfig.CacheTTL)
		if err != nil {
			log.Warnf("Invalid JWT cache TTL %q, using default (%d)", argoSettings.JWTConfig.CacheTTL, cacheTTL)
		} else {
			cacheTTL = ttl
		}
	}

	jwks, err := p.getJWKS(ctx, argoSettings.JWTConfig.JWKSetURL, cacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	// Determine signing algorithm, default to RS256 if not set
	allowedSigningAlg := "RS256"
	if argoSettings.JWTConfig.SigningAlgorithm != "" {
		allowedSigningAlg = argoSettings.JWTConfig.SigningAlgorithm
	}

	// --- Key Function ---
	keyFunc := func(token *jwtgo.Token) (any, error) {
		// Ensure the signing method is expected before continuing.
		// The WithValidMethods option below enforces this, but double-checking here is fine.
		if token.Method.Alg() != allowedSigningAlg {
			return nil, fmt.Errorf("unexpected signing algorithm: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found in token")
		}

		var key *jose.JSONWebKey
		for _, k := range jwks.Keys {
			if k.KeyID == kid {
				key = &k
				break
			}
		}
		if key == nil {
			return nil, fmt.Errorf("no key found for kid %q", kid)
		}

		if key.Algorithm != "" && key.Algorithm != token.Header["alg"] {
			return nil, fmt.Errorf("algorithm mismatch for kid %q: expected %v, got %v. JWT issuer may be misconfigured/broken", kid, key.Algorithm, token.Header["alg"])
		}

		return key.Key, nil
	}
	// --- End Key Function ---

	// --- Parser Options ---
	opts := []jwtgo.ParserOption{
		jwtgo.WithValidMethods([]string{allowedSigningAlg}), // Enforce expected signing algorithm
		// Add other standard validation options based on config
	}
	if argoSettings.JWTConfig.Issuer != "" {
		opts = append(opts, jwtgo.WithIssuer(argoSettings.JWTConfig.Issuer))
	}
	if argoSettings.JWTConfig.Audience != "" {
		opts = append(opts, jwtgo.WithAudience(argoSettings.JWTConfig.Audience))
	}
	// By default, Parse validates exp, nbf, iat. Add options if specific behavior is needed.
	// opts = append(opts, jwtgo.WithExpirationRequired()) // Uncomment if expiration MUST be present
	// opts = append(opts, jwtgo.WithIssuedAt()) // Enforces iat check
	// --- End Parser Options ---

	// --- Parse and Validate ---
	parser := jwtgo.NewParser(opts...)
	token, err := parser.Parse(tokenString, keyFunc)
	if err != nil {
		// Log the specific parsing/verification error for better debugging
		log.Debugf("JWT parsing/verification failed: %v", err)
		// Check for specific validation errors if needed for more context
		if errors.Is(err, jwtgo.ErrTokenInvalidIssuer) {
			return nil, fmt.Errorf("invalid issuer claim: %w", err)
		}
		if errors.Is(err, jwtgo.ErrTokenInvalidAudience) {
			return nil, fmt.Errorf("invalid audience claim: %w", err)
		}
		if errors.Is(err, jwtgo.ErrTokenExpired) {
			return nil, fmt.Errorf("token is expired: %w", err)
		}
		// Return a generic error for other parsing/signature issues
		return nil, fmt.Errorf("failed to parse/verify JWT: %w", err)
	}
	// --- End Parse and Validate ---

	// --- Custom Claim Checks ---
	claims, ok := token.Claims.(jwtgo.MapClaims)
	if !ok {
		// This should ideally not happen if parsing succeeded, but check anyway.
		return nil, errors.New("invalid token claims format after successful parse")
	}

	if argoSettings.JWTConfig.EmailClaim != "" {
		if _, ok := claims[argoSettings.JWTConfig.EmailClaim]; !ok {
			log.Warnf("Required email claim %q not found in JWT", argoSettings.JWTConfig.EmailClaim)
			// Depending on requirements, you might want to return an error here instead of just logging.
			// For now, let's allow it but log a warning.
			// return nil, fmt.Errorf("required email claim %q not found", argoSettings.JWTConfig.EmailClaim)
		}
	}

	if argoSettings.JWTConfig.UsernameClaim != "" {
		if _, ok := claims[argoSettings.JWTConfig.UsernameClaim]; !ok {
			log.Warnf("Required username claim %q not found in JWT", argoSettings.JWTConfig.UsernameClaim)
			// Depending on requirements, you might want to return an error here instead of just logging.
			// For now, let's allow it but log a warning.
			// return nil, fmt.Errorf("required username claim %q not found", argoSettings.JWTConfig.UsernameClaim)
		}
	}

	// Verify audience if configured
	if argoSettings.JWTConfig.Audience != "" {
		audience, err := claims.GetAudience()
		if err != nil {
			// Consider if audience claim is mandatory based on your policy
			// return nil, fmt.Errorf("failed to get audience claim: %w", err)
			log.Debugf("Failed to get audience claim, continuing verification: %v", err)
		} else {
			validAud := false
			for _, aud := range audience {
				if aud == argoSettings.JWTConfig.Audience {
					validAud = true
					break
				}
			}
			if !validAud {
				return nil, fmt.Errorf("invalid audience claim, expected aud %q not found in %v. Perhaps someone is trying to use a token from a different issuer", argoSettings.JWTConfig.Audience, audience)
			}
		}
	}

	// Parse groups and set claim for later handling at "groups" scope
	if argoSettings.JWTConfig.GroupsClaim != "" {
		if groups, ok := getNestedClaim(claims, argoSettings.JWTConfig.GroupsClaim); ok {
			// groups should be an array of strings...
			if groupsSlice, ok := groups.([]any); ok {
				stringGroups := make([]string, 0, len(groupsSlice))
				for _, group := range groupsSlice {
					if groupStr, ok := group.(string); ok {
						stringGroups = append(stringGroups, groupStr)
					}
				}
				claims["groups"] = stringGroups
			}
		} else {
			log.Warnf("Groups claim %q not found in JWT", argoSettings.JWTConfig.GroupsClaim)
		}
	}

	// --- End Custom Claim Checks ---

	return token, nil
}

func (p *providerImpl) getJWKS(ctx context.Context, jwksURL string, cacheTTL time.Duration) (*jose.JSONWebKeySet, error) {
	p.jwksCacheMux.Lock()
	defer p.jwksCacheMux.Unlock()

	if p.jwksCache != nil && time.Now().Before(p.jwksExpiry) {
		return p.jwksCache, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	var jwks jose.JSONWebKeySet
	err = json.NewDecoder(resp.Body).Decode(&jwks)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	p.jwksCache = &jwks
	p.jwksExpiry = time.Now().Add(cacheTTL)

	return &jwks, nil
}

func (p *providerImpl) verify(ctx context.Context, clientID, tokenString string, skipClientIDCheck bool) (*gooidc.IDToken, error) {
	prov, err := p.provider()
	if err != nil {
		return nil, err
	}
	config := &gooidc.Config{ClientID: clientID, SkipClientIDCheck: skipClientIDCheck}
	verifier := prov.Verifier(config)
	idToken, err := verifier.Verify(ctx, tokenString)
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
		newProvider, retryErr := p.newGoOIDCProvider()
		if retryErr != nil {
			// return original error if we fail to re-initialize OIDC
			return nil, err
		}
		verifier = newProvider.Verifier(config)
		idToken, err = verifier.Verify(ctx, tokenString)
		if err != nil {
			return nil, err
		}
		// If we get here, we successfully re-initialized OIDC and after re-initialization,
		// the token is now valid.
		log.Info("New OIDC settings detected")
		p.goOIDCProvider = newProvider
	}
	return idToken, nil
}

func (p *providerImpl) Endpoint() (*oauth2.Endpoint, error) {
	prov, err := p.provider()
	if err != nil {
		return nil, err
	}
	endpoint := prov.Endpoint()
	return &endpoint, nil
}

// ParseConfig parses the OIDC Config into the concrete datastructure
func (p *providerImpl) ParseConfig() (*OIDCConfiguration, error) {
	prov, err := p.provider()
	if err != nil {
		return nil, err
	}
	return ParseConfig(prov)
}

// ParseConfig parses the OIDC Config into the concrete datastructure
func ParseConfig(provider *gooidc.Provider) (*OIDCConfiguration, error) {
	var conf OIDCConfiguration
	err := provider.Claims(&conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

// getNestedClaim retrieves a value from a nested map using a dot-separated path.
// For example, given path "user.profile.name", it will traverse:
// data["user"]["profile"]["name"]
// Returns the value and true if found, nil and false otherwise.
func getNestedClaim(data map[string]any, path string) (any, bool) {
	keys := strings.Split(path, ".")
	var current any = data

	for i, key := range keys {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		value, exists := currentMap[key]
		if !exists {
			return nil, false
		}

		if i == len(keys)-1 {
			return value, true
		}
		current = value
	}
	return nil, false
}
