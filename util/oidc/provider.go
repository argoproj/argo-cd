package oidc

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gooidc "github.com/coreos/go-oidc"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
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

	Verify(clientID, tokenString string) (*gooidc.IDToken, error)
}

type providerImpl struct {
	issuerURL      string
	client         *http.Client
	goOIDCProvider *gooidc.Provider
}

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
		return nil, fmt.Errorf("Failed to query provider %q: %v", p.issuerURL, err)
	}
	s, _ := ParseConfig(prov)
	log.Infof("OIDC supported scopes: %v", s.ScopesSupported)
	return prov, nil
}

func (p *providerImpl) Verify(clientID, tokenString string) (*gooidc.IDToken, error) {
	ctx := context.Background()
	prov, err := p.provider()
	if err != nil {
		return nil, err
	}
	verifier := prov.Verifier(&gooidc.Config{ClientID: clientID})
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
		verifier = newProvider.Verifier(&gooidc.Config{ClientID: clientID})
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
