package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/helm"
)

func TestGetGitCredsShouldReturnAzureWorkloadIdentityCredsIfSpecified(t *testing.T) {
	repository := Repository{UseAzureWorkloadIdentity: true}

	creds := repository.GetGitCreds(git.NoopCredsStore{})

	_, ok := creds.(git.AzureWorkloadIdentityCreds)
	require.Truef(t, ok, "expected AzureWorkloadIdentityCreds but got %T", creds)
}

func TestGetHelmCredsShouldReturnAzureWorkloadIdentityCredsIfSpecified(t *testing.T) {
	repository := Repository{UseAzureWorkloadIdentity: true}

	creds := repository.GetHelmCreds()

	_, ok := creds.(helm.AzureWorkloadIdentityCreds)
	require.Truef(t, ok, "expected AzureWorkloadIdentityCreds but got %T", creds)
}

func TestGetHelmCredsShouldReturnHelmCredsIfAzureWorkloadIdentityNotSpecified(t *testing.T) {
	repository := Repository{}

	creds := repository.GetHelmCreds()

	_, ok := creds.(helm.HelmCreds)
	require.Truef(t, ok, "expected HelmCreds but got %T", creds)
}

func TestGetGitCreds(t *testing.T) {
	tests := []struct {
		name     string
		repo     *Repository
		expected git.Creds
	}{
		{
			name:     "nil repository",
			repo:     nil,
			expected: git.NopCreds{},
		},
		{
			name: "HTTPS credentials",
			repo: &Repository{
				Username: "user",
				Password: "pass",
			},
			expected: git.NewHTTPSCreds("user", "pass", "", "", "", false, nil, false),
		},
		{
			name: "Bearer token credentials",
			repo: &Repository{
				BearerToken: "token",
			},
			expected: git.NewHTTPSCreds("", "", "token", "", "", false, nil, false),
		},
		{
			name: "SSH credentials",
			repo: &Repository{
				SSHPrivateKey: "ssh-key",
			},
			expected: git.NewSSHCreds("ssh-key", "", false, ""),
		},
		{
			name: "GitHub App credentials",
			repo: &Repository{
				GithubAppPrivateKey:     "github-key",
				GithubAppId:             123,
				GithubAppInstallationId: 456,
			},
			expected: git.NewGitHubAppCreds(123, 456, "github-key", "", "", "", false, "", "", nil),
		},
		{
			name: "Google Cloud credentials",
			repo: &Repository{
				GCPServiceAccountKey: "gcp-key",
			},
			expected: git.NewGoogleCloudCreds("gcp-key", nil),
		},
		{
			name:     "No credentials",
			repo:     &Repository{},
			expected: git.NopCreds{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := tt.repo.GetGitCreds(nil)
			assert.Equal(t, tt.expected, creds)
		})
	}
}

func TestGetGitCreds_GitHubApp_InstallationNotFound(t *testing.T) {
	// This test verifies that when GitHub App credentials are provided but the installation
	// cannot be discovered (e.g., non-existent org), GetGitCreds returns GitHubAppCreds with
	// installation ID 0. When that credential is used, it should provide a clear error message
	// indicating the app is not installed, rather than a confusing "404 Not Found" error.
	repo := &Repository{
		Repo:                "https://github.com/nonexistent-org-12345/repo.git",
		GithubAppPrivateKey: "github-key",
		GithubAppId:         123,
		// GithubAppInstallationId is 0 (not set), triggering auto-discovery
	}

	creds := repo.GetGitCreds(nil)

	// We should still get GitHubAppCreds (not NopCreds)
	ghAppCreds, isGitHubAppCreds := creds.(git.GitHubAppCreds)
	require.True(t, isGitHubAppCreds, "expected GitHubAppCreds when installation discovery fails, got %T", creds)

	// When we try to use these credentials, we should get a clear error about installation ID 0
	_, _, err := ghAppCreds.Environ()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "installation ID is 0")
	assert.Contains(t, err.Error(), "app is not installed")
}

func TestGetGitCreds_GitHubApp_OrgExtractionFails(t *testing.T) {
	// This test verifies that when the organization cannot be extracted from the repo URL,
	// GetGitCreds still returns GitHubAppCreds with installation ID 0, which will provide
	// a clear error message when used.
	repo := &Repository{
		Repo:                "invalid-url-format",
		GithubAppPrivateKey: "github-key",
		GithubAppId:         123,
		// GithubAppInstallationId is 0 (not set), triggering auto-discovery
	}

	creds := repo.GetGitCreds(nil)

	// We should still get GitHubAppCreds
	ghAppCreds, isGitHubAppCreds := creds.(git.GitHubAppCreds)
	require.True(t, isGitHubAppCreds, "expected GitHubAppCreds when org extraction fails, got %T", creds)

	// When we try to use these credentials, we should get a clear error about installation ID 0
	_, _, err := ghAppCreds.Environ()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "installation ID is 0")
	assert.Contains(t, err.Error(), "app is not installed")
}
