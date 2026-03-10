package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// backchannelLogoutEventType is the required key in the "events" claim of an OIDC backchannel logout token,
// as specified in https://openid.net/specs/openid-connect-backchannel-1_0.html#LogoutToken.
const backchannelLogoutEventType = "http://schemas.openid.net/event/backchannel-logout"

// LogoutTokenClaims holds the verified claims extracted from an OIDC backchannel logout token.
type LogoutTokenClaims struct {
	// Sub is the subject identifier (user ID) from the logout token.
	Sub string
	// Sid is the OIDC session ID that should be revoked.
	Sid string
}

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

	// VerifyLogoutToken verifies an OIDC backchannel logout token. It validates the token signature,
	// standard JWT claims (iss, aud, iat, exp), the required "events" claim, and the absence of a
	// "nonce" claim. It returns the relevant claims on success.
	VerifyLogoutToken(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (*LogoutTokenClaims, error)
}

type providerImpl struct {
	issuerURL      string
	client         *http.Client
	goOIDCProvider *gooidc.Provider
}

var _ Provider = &providerImpl{}

// NewOIDCProvider initializes an OIDC provider
func NewOIDCProvider(issuerURL string, client *http.Client) Provider {
	return &providerImpl{
		issuerURL: issuerURL,
		client:    client,
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
	var span trace.Span
	ctx, span = tracer.Start(ctx, "oidc.providerImpl.Verify")
	defer span.End()
	unverifiedHasAudClaim, err := security.UnverifiedHasAudClaim(tokenString)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to determine whether the token has an aud claim: %w", err)
	}

	var idToken *gooidc.IDToken
	if !unverifiedHasAudClaim {
		idToken, err = p.verify(ctx, "", tokenString, argoSettings.SkipAudienceCheckWhenTokenHasNoAudience())
	} else {
		allowedAudiences := argoSettings.OAuth2AllowedAudiences()
		span.SetAttributes(attribute.StringSlice("allowedAudiences", allowedAudiences))
		if len(allowedAudiences) == 0 {
			span.SetStatus(codes.Error, "token has an audience claim, but no allowed audiences are configured")
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
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to verify provider token: %w", err)
	}

	return idToken, nil
}

func (p *providerImpl) verify(ctx context.Context, clientID, tokenString string, skipClientIDCheck bool) (*gooidc.IDToken, error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, "oidc.providerImpl.verify")
	defer span.End()
	prov, err := p.provider()
	if err != nil {
		span.SetStatus(codes.Error, fmt.Sprintf("failed to query provider: %v", err))
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
			span.SetStatus(codes.Error, fmt.Sprintf("error verifying token: %v", err))
			return nil, err
		}
		newProvider, retryErr := p.newGoOIDCProvider()
		if retryErr != nil {
			span.SetStatus(codes.Error, fmt.Sprintf("hack: error verifying token on retry: %v", err))
			// return original error if we fail to re-initialize OIDC
			return nil, err
		}
		verifier = newProvider.Verifier(config)
		idToken, err = verifier.Verify(ctx, tokenString)
		if err != nil {
			span.SetStatus(codes.Error, fmt.Sprintf("hack: error verifying token: %v", err))
			return nil, err
		}
		// If we get here, we successfully re-initialized OIDC and after re-initialization,
		// the token is now valid.
		log.Info("New OIDC settings detected")
		p.goOIDCProvider = newProvider
	}
	return idToken, nil
}

// verifyLogoutTokenSignature verifies the signature, issuer, and audience of a logout token using
// a verifier with SkipExpiryCheck: true. Per the OIDC Backchannel Logout spec (section 2.4), the
// "exp" claim is OPTIONAL in logout tokens, so we must not reject tokens that omit it. When "exp"
// is present we enforce it ourselves after this call.
func (p *providerImpl) verifyLogoutTokenSignature(ctx context.Context, clientID, tokenString string, skipClientIDCheck bool) (*gooidc.IDToken, error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, "oidc.providerImpl.verifyLogoutTokenSignature")
	defer span.End()
	prov, err := p.provider()
	if err != nil {
		span.SetStatus(codes.Error, fmt.Sprintf("failed to query provider: %v", err))
		return nil, err
	}
	config := &gooidc.Config{
		ClientID:          clientID,
		SkipClientIDCheck: skipClientIDCheck,
		// exp is OPTIONAL in logout tokens (OIDC Back-Channel Logout spec §2.4).
		// We enforce expiry ourselves below when the claim is present.
		SkipExpiryCheck: true,
	}
	verifier := prov.Verifier(config)
	idToken, err := verifier.Verify(ctx, tokenString)
	if err != nil {
		// Same JWKS-rotation hack as in verify().
		if !strings.Contains(err.Error(), "failed to verify signature") {
			span.SetStatus(codes.Error, fmt.Sprintf("error verifying logout token: %v", err))
			return nil, err
		}
		newProvider, retryErr := p.newGoOIDCProvider()
		if retryErr != nil {
			span.SetStatus(codes.Error, fmt.Sprintf("hack: logout token retry failed: %v", err))
			return nil, err
		}
		verifier = newProvider.Verifier(config)
		idToken, err = verifier.Verify(ctx, tokenString)
		if err != nil {
			span.SetStatus(codes.Error, fmt.Sprintf("hack: logout token retry error: %v", err))
			return nil, err
		}
		log.Info("New OIDC settings detected (logout token path)")
		p.goOIDCProvider = newProvider
	}
	return idToken, nil
}

// VerifyLogoutToken implements Provider.
func (p *providerImpl) VerifyLogoutToken(ctx context.Context, tokenString string, argoSettings *settings.ArgoCDSettings) (*LogoutTokenClaims, error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, "oidc.providerImpl.VerifyLogoutToken")
	defer span.End()

	// Validate signature, issuer, and audience. exp is checked separately below since it
	// is optional in logout tokens per the OIDC Backchannel Logout spec (section 2.4).
	var (
		idToken *gooidc.IDToken
		err     error
	)
	unverifiedHasAudClaim, err := security.UnverifiedHasAudClaim(tokenString)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to determine whether the logout token has an aud claim: %w", err)
	}
	if !unverifiedHasAudClaim {
		idToken, err = p.verifyLogoutTokenSignature(ctx, "", tokenString, argoSettings.SkipAudienceCheckWhenTokenHasNoAudience())
	} else {
		allowedAudiences := argoSettings.OAuth2AllowedAudiences()
		if len(allowedAudiences) == 0 {
			err = errors.New("logout token has an audience claim, but no allowed audiences are configured")
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		tokenVerificationErrors := make(map[string]error)
		for _, aud := range allowedAudiences {
			idToken, err = p.verifyLogoutTokenSignature(ctx, aud, tokenString, false)
			if err == nil {
				break
			}
			tokenVerificationErrors[aud] = err
		}
		if err != nil && len(tokenVerificationErrors) > 0 {
			err = tokenVerificationError{errorsByAudience: tokenVerificationErrors}
		}
	}
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to verify logout token: %w", err)
	}

	// Enforce expiry manually: if exp is set (non-zero) it must not be in the past.
	if !idToken.Expiry.IsZero() && time.Now().After(idToken.Expiry) {
		err = fmt.Errorf("logout token is expired (exp: %s)", idToken.Expiry)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to parse logout token claims: %w", err)
	}

	// Per spec: logout tokens MUST NOT contain a nonce claim.
	if _, hasNonce := rawClaims["nonce"]; hasNonce {
		err := errors.New("logout token must not contain a nonce claim")
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Per spec: logout tokens MUST contain an events claim whose value is a JSON object
	// containing the member name backchannelLogoutEventType with an empty JSON object as its value.
	eventsRaw, ok := rawClaims["events"]
	if !ok {
		err := errors.New("logout token missing required events claim")
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	eventsMap, ok := eventsRaw.(map[string]any)
	if !ok {
		err := errors.New("logout token events claim has unexpected type")
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if _, ok := eventsMap[backchannelLogoutEventType]; !ok {
		err := fmt.Errorf("logout token events claim does not contain required %q key", backchannelLogoutEventType)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Per spec: logout tokens MUST contain either a sub claim, a sid claim, or both.
	// We require sid since we use it as the session identifier for revocation.
	sid, _ := rawClaims["sid"].(string)
	if sid == "" {
		err := errors.New("logout token missing required sid claim")
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	sub, _ := rawClaims["sub"].(string)

	span.SetAttributes(
		attribute.String("sub", sub),
		attribute.String("sid", sid),
	)
	return &LogoutTokenClaims{Sub: sub, Sid: sid}, nil
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
