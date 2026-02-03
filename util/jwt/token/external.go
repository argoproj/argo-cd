package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

	if argoSettings.JWTConfig.EmailClaim != "" {
		if _, ok := claims[argoSettings.JWTConfig.EmailClaim]; !ok {
			log.Warnf("Required email claim %q not found in external JWT", argoSettings.JWTConfig.EmailClaim)
			// Depending on requirements, you might want to return an error here instead of just logging.
			// For now, let's allow it but log a warning.
			// return nil, fmt.Errorf("required email claim %q not found", argoSettings.JWTConfig.EmailClaim)
		}
	}

	if argoSettings.JWTConfig.UsernameClaim != "" {
		if _, ok := claims[argoSettings.JWTConfig.UsernameClaim]; !ok {
			log.Warnf("Required username claim %q not found in external JWT", argoSettings.JWTConfig.UsernameClaim)
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
			log.Debugf("Failed to get audience claim from external JWT, continuing verification: %v", err)
		} else {
			validAud := false
			for _, aud := range audience {
				if aud == argoSettings.JWTConfig.Audience {
					validAud = true
					break
				}
			}
			if !validAud {
				return nil, fmt.Errorf("invalid audience claim in external JWT, expected aud %q not found in %v. Perhaps someone is trying to use a token from a different issuer", argoSettings.JWTConfig.Audience, audience)
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

	return claims, nil
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
