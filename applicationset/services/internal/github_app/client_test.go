package github_app

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/stretchr/testify/assert"
)

func TestResolveAPIURLPrefersCredentialEnterpriseURL(t *testing.T) {
	enterpriseURL := "https://api.example.ghe.com"
	configuredURL := "https://api.other.example.com"

	assert.Equal(t, enterpriseURL, resolveAPIURL(github_app_auth.Authentication{
		EnterpriseBaseURL: enterpriseURL,
	}, configuredURL))
	assert.Equal(t, configuredURL, resolveAPIURL(github_app_auth.Authentication{}, configuredURL))
}
