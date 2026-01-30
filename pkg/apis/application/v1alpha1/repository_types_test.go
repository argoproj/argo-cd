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

	creds, err := repository.GetGitCreds(git.NoopCredsStore{})
	require.NoError(t, err)

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
		name        string
		repo        *Repository
		expected    git.Creds
		expectError bool
	}{
		{
			name:        "nil repository",
			repo:        nil,
			expected:    git.NopCreds{},
			expectError: false,
		},
		{
			name: "HTTPS credentials",
			repo: &Repository{
				Username: "user",
				Password: "pass",
			},
			expected:    git.NewHTTPSCreds("user", "pass", "", "", "", false, nil, false),
			expectError: false,
		},
		{
			name: "Bearer token credentials",
			repo: &Repository{
				BearerToken: "token",
			},
			expected:    git.NewHTTPSCreds("", "", "token", "", "", false, nil, false),
			expectError: false,
		},
		{
			name: "SSH credentials",
			repo: &Repository{
				SSHPrivateKey: "ssh-key",
			},
			expected:    git.NewSSHCreds("ssh-key", "", false, ""),
			expectError: false,
		},
		{
			name: "GitHub App credentials",
			repo: &Repository{
				GithubAppPrivateKey:     "github-key",
				GithubAppId:             123,
				GithubAppInstallationId: 456,
			},
			expected:    git.NewGitHubAppCreds(123, 456, "github-key", "", "", "", false, "", "", nil),
			expectError: false,
		},
		{
			name: "Google Cloud credentials",
			repo: &Repository{
				GCPServiceAccountKey: "gcp-key",
			},
			expected:    git.NewGoogleCloudCreds("gcp-key", nil),
			expectError: false,
		},
		{
			name:        "No credentials",
			repo:        &Repository{},
			expected:    git.NopCreds{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := tt.repo.GetGitCreds(nil)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, creds)
			}
		})
	}
}

func TestGetGitCreds_GitHubApp_InstallationNotFound(t *testing.T) {
	// This test verifies that when GitHub App credentials are provided but the installation
	// cannot be discovered (e.g., non-existent org), GetGitCreds returns a clear error message
	// indicating the app is not installed, rather than proceeding with installation ID 0.
	repo := &Repository{
		Repo:                "https://github.com/nonexistent-org-12345/repo.git",
		GithubAppPrivateKey: "github-key",
		GithubAppId:         123,
		// GithubAppInstallationId is 0 (not set), triggering auto-discovery
	}

	creds, err := repo.GetGitCreds(nil)

	require.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "failed to discover GitHub App installation ID")
	assert.Contains(t, err.Error(), "nonexistent-org-12345")
	assert.Contains(t, err.Error(), "ID: 123")
}

func TestGetGitCreds_GitHubApp_OrgExtractionFails(t *testing.T) {
	// This test verifies that when the organization cannot be extracted from the repo URL,
	// GetGitCreds returns a clear error about the URL format issue.
	repo := &Repository{
		Repo:                "invalid-url-format",
		GithubAppPrivateKey: "github-key",
		GithubAppId:         123,
		// GithubAppInstallationId is 0 (not set), triggering auto-discovery
	}

	creds, err := repo.GetGitCreds(nil)

	require.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "failed to extract organization")
	assert.Contains(t, err.Error(), "invalid-url-format")
}
