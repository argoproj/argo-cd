package oidc

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

var (
	spaOauth2Conf = &oauth2.Config{
		ClientID: "spa-id",
	}

	webOauth2Conf = &oauth2.Config{
		ClientID:     "spa-id",
		ClientSecret: "my-super-secret",
	}
)

func TestInferGrantType(t *testing.T) {
	var grantType string
	dexRAW, err := ioutil.ReadFile("testdata/dex.json")
	assert.NoError(t, err)
	var dexConfig OIDCConfiguration
	err = json.Unmarshal(dexRAW, &dexConfig)
	assert.NoError(t, err)
	grantType = InferGrantType(spaOauth2Conf, &dexConfig)
	// Dex does not support implicit login flow (https://github.com/dexidp/dex/issues/1254)
	assert.Equal(t, GrantTypeAuthorizationCode, grantType)
	grantType = InferGrantType(webOauth2Conf, &dexConfig)
	assert.Equal(t, GrantTypeAuthorizationCode, grantType)

	testFiles := []string{"testdata/okta.json", "testdata/auth0.json", "testdata/onelogin.json"}
	for _, path := range testFiles {
		oktaRAW, err := ioutil.ReadFile(path)
		assert.NoError(t, err)
		var oktaConfig OIDCConfiguration
		err = json.Unmarshal(oktaRAW, &oktaConfig)
		assert.NoError(t, err)
		grantType = InferGrantType(spaOauth2Conf, &oktaConfig)
		assert.Equal(t, GrantTypeImplicit, grantType)
		grantType = InferGrantType(webOauth2Conf, &oktaConfig)
		assert.Equal(t, GrantTypeAuthorizationCode, grantType)
	}
}

func TestGetScopes(t *testing.T) {
	var oidcConfig OIDCConfiguration
	oidcConfig.ScopesSupported = []string{"openid", "profile", "email", "groups"}
	var scopes []string
	scopes = GetScopes(&oidcConfig)
	assert.Equal(t, []string{"openid", "profile", "email", "groups"}, scopes)
	// AWS cognito does not support the groups scope test this case
	oidcConfig.ScopesSupported = []string{"openid", "profile", "email"}
	scopes = GetScopes(&oidcConfig)
	assert.Equal(t, []string{"openid", "profile", "email"}, scopes)

}
