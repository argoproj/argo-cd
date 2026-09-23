package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	jwtgo "github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

type externalTokenVerifier struct {
	client *http.Client

	jwksCache       *jose.JSONWebKeySet
	jwksExpiry      time.Time
	jwksCacheMux    sync.Mutex
	defaultCacheTTL time.Duration
}

func NewExternalTokenVerifier(client *http.Client) Verifier {
	return &externalTokenVerifier{
		client:          client,
		defaultCacheTTL: 5 * time.Minute,
	}
}

// VerifyToken verifies an externally injected JWT token using the configured JWK Set URL
func (v *externalTokenVerifier) Verify(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (jwtgo.Claims, error) {
	if !argoSettings.IsJWTConfigured() {
		return nil, errors.New("valid JWT configuration not found")
	}

	cacheTTL := v.defaultCacheTTL
	if argoSettings.JWTConfig.CacheTTL != "" {
		ttl, err := time.ParseDuration(argoSettings.JWTConfig.CacheTTL)
		if err != nil {
			log.Warnf("Invalid JWT cache TTL %q, using default (%d)", argoSettings.JWTConfig.CacheTTL, cacheTTL)
		} else {
			cacheTTL = ttl
		}
	}

	jwks, err := v.getJWKS(ctx, argoSettings.JWTConfig.JWKSetURL, cacheTTL)
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
			return nil, fmt.Errorf("unexpected signing algorithm in external JWT: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found in external JWT")
		}

		var key *jose.JSONWebKey
		for _, k := range jwks.Keys {
			if k.KeyID == kid {
				key = &k
				break
			}
		}
		if key == nil {
			return nil, fmt.Errorf("no key found for kid in external JWT: %q", kid)
		}

		if key.Algorithm != "" && key.Algorithm != token.Header["alg"] {
			return nil, fmt.Errorf("algorithm mismatch for kid %q: expected %v, got %v. External JWT issuer may be misconfigured/broken", kid, key.Algorithm, token.Header["alg"])
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
		log.Debugf("externalJWT parsing/verification failed: %v", err)
		// Check for specific validation errors if needed for more context
		if errors.Is(err, jwtgo.ErrTokenInvalidIssuer) {
			return nil, fmt.Errorf("invalid issuer claim in external JWT: %w", err)
		}
		if errors.Is(err, jwtgo.ErrTokenInvalidAudience) {
			return nil, fmt.Errorf("invalid audience claim in external JWT: %w", err)
		}
		if errors.Is(err, jwtgo.ErrTokenExpired) {
			return nil, fmt.Errorf("external JWT is expired: %w", err)
		}
		// Return a generic error for other parsing/signature issues
		return nil, fmt.Errorf("failed to parse/verify external JWT: %w", err)
	}
	// --- End Parse and Validate ---

	// --- Custom Claim Checks ---
	claims, ok := token.Claims.(jwtgo.MapClaims)
	if !ok {
		// This should ideally not happen if parsing succeeded, but check anyway.
		return nil, errors.New("invalid external JWT claims format after successful parse")
	}

	// Verify audience if configured
	if argoSettings.JWTConfig.Audience != "" {
		audience, err := claims.GetAudience()
		if err != nil {
			// Consider if audience claim is mandatory based on your policy
			// return nil, fmt.Errorf("failed to get audience claim: %w", err)
			log.Debugf("Failed to get audience claim from external JWT, continuing verification: %v", err)
		} else if !slices.Contains(audience, argoSettings.JWTConfig.Audience) {
			return nil, fmt.Errorf("invalid audience claim in external JWT, expected aud %q not found in %v. Perhaps someone is trying to use a token from a different issuer", argoSettings.JWTConfig.Audience, audience)
		}
	}

	normalizeClaims(claims, argoSettings.JWTConfig)

	// --- End Custom Claim Checks ---

	return claims, nil
}

// normalizeClaims projects the claims named in the JWT config onto the claims Argo CD reads
// downstream: "sub" (jwt.GetUserIdentifier, and therefore the RBAC subject), "email" (the
// displayed username) and "groups" (matched against the RBAC "scopes" setting).
//
// Claims the config does not name are left exactly as the issuer sent them. The single
// exception is groups: a groupsClaim that is set but does not resolve removes "groups", since
// otherwise a misconfigured groupsClaim silently falls back to the issuer's own "groups" claim
// and the setting has no observable effect.
func normalizeClaims(claims jwtgo.MapClaims, config *settings.JWTConfig) {
	// Resolve everything before writing, so identity mappings (usernameClaim: sub,
	// groupsClaim: groups) read the original value rather than one we just overwrote.
	username, hasUsername := getNestedClaimString(claims, config.UsernameClaim)
	email, hasEmail := getNestedClaimString(claims, config.EmailClaim)
	groups, hasGroups := getNestedClaimStrings(claims, config.GroupsClaim)

	if config.UsernameClaim != "" {
		if hasUsername {
			// NOTE: jwt.GetUserIdentifier prefers federated_claims.user_id over sub, so a token
			// carrying that claim would outrank this mapping. federated_claims is emitted by Dex,
			// which cannot be the verifier when external JWT is configured, so it is left alone
			// rather than stripped here. Fix GetUserIdentifier if an issuer ever does send it.
			claims["sub"] = username
		} else {
			log.Warnf("Username claim %q not found in external JWT, falling back to the sub claim", config.UsernameClaim)
		}
	}

	if config.EmailClaim != "" {
		if hasEmail {
			claims["email"] = email
		} else {
			log.Warnf("Email claim %q not found in external JWT", config.EmailClaim)
		}
	}

	switch {
	case hasGroups:
		claims["groups"] = groups
	case config.GroupsClaim != "":
		// Configured but unresolvable. Remove any "groups" the issuer sent: keeping it would
		// silently grant RBAC group membership from a claim the operator did not select.
		log.Warnf("Groups claim %q not found in external JWT, the user will be assigned the default role", config.GroupsClaim)
		delete(claims, "groups")
	}
}

func (v *externalTokenVerifier) getJWKS(ctx context.Context, jwksURL string, cacheTTL time.Duration) (*jose.JSONWebKeySet, error) {
	v.jwksCacheMux.Lock()
	defer v.jwksCacheMux.Unlock()

	if v.jwksCache != nil && time.Now().Before(v.jwksExpiry) {
		return v.jwksCache, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	var jwks jose.JSONWebKeySet
	err = json.NewDecoder(resp.Body).Decode(&jwks)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	v.jwksCache = &jwks
	v.jwksExpiry = time.Now().Add(cacheTTL)

	log.Debug("Token verified using JWT")
	return &jwks, nil
}

// getNestedClaim retrieves a value from a nested map using a dot-separated path.
// For example, given path "user.profile.name", it will traverse:
// data["user"]["profile"]["name"]
// Returns the value and true if found, nil and false otherwise.
//
// The whole path is tried as a literal key first, because claim names may contain dots
// themselves. Issuers that namespace custom claims as URIs produce names like
// "https://argocd.example.com/groups", which the dot-separated syntax cannot otherwise
// express. A literal match therefore wins over traversal.
func getNestedClaim(data map[string]any, path string) (any, bool) {
	if value, exists := data[path]; exists {
		return value, true
	}

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

// getNestedClaimString resolves a claim path to a non-empty string. An empty path, a
// missing claim or a non-string value all return false.
func getNestedClaimString(claims map[string]any, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	value, ok := getNestedClaim(claims, path)
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	if !ok || str == "" {
		return "", false
	}
	return str, true
}

// getNestedClaimStrings resolves a claim path to a list of strings. Issuers spell
// list-valued claims in several ways, so a JSON array (decoded as []any), a []string and a
// lone string are all accepted. An empty path, a missing claim or any other value return
// false.
func getNestedClaimStrings(claims map[string]any, path string) ([]string, bool) {
	if path == "" {
		return nil, false
	}
	value, ok := getNestedClaim(claims, path)
	if !ok {
		return nil, false
	}

	switch typed := value.(type) {
	case string:
		return []string{typed}, true
	case []string:
		return typed, true
	case []any:
		strs := make([]string, 0, len(typed))
		for _, item := range typed {
			str, ok := item.(string)
			if !ok {
				log.Warnf("Ignoring non-string entry of type %T in claim %q of external JWT", item, path)
				continue
			}
			strs = append(strs, str)
		}
		return strs, true
	default:
		log.Warnf("Claim %q in external JWT has unsupported type %T, expected a string or a list of strings", path, value)
		return nil, false
	}
}
