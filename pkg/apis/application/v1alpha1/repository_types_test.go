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
				Repo:                    "https://github.com/argoproj/argo-cd",
				GithubAppPrivateKey:     "github-key",
				GithubAppId:             123,
				GithubAppInstallationId: 456,
			},
			expected: git.NewGitHubAppCreds(123, 456, "github-key", "", "", "", false, "", "", nil, "https://github.com/argoproj/argo-cd"),
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
	// cannot be discovered (e.g., non-existent org), the error is raised when the credentials
	// are used (lazily), providing a clear error message.
	repo := &Repository{
		Repo:                "https://github.com/nonexistent-org-12345/repo.git",
		GithubAppPrivateKey: "github-key",
		GithubAppId:         123,
		// GithubAppInstallationId is 0 (not set), triggering auto-discovery
	}

	creds := repo.GetGitCreds(nil)

	// We should get GitHubAppCreds
	ghAppCreds, isGitHubAppCreds := creds.(git.GitHubAppCreds)
	require.True(t, isGitHubAppCreds, "expected GitHubAppCreds, got %T", creds)

	// When we try to use these credentials, we should get a clear error about installation discovery failure
	_, _, err := ghAppCreds.Environ()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to discover GitHub App installation ID")
	assert.Contains(t, err.Error(), "nonexistent-org-12345")
	assert.Contains(t, err.Error(), "ID: 123")
}

func TestSanitizedRepository(t *testing.T) {
	repo := &Repository{
		Repo:                       "https://github.com/argoproj/argo-cd.git",
		Type:                       "git",
		Name:                       "argo-cd",
		Username:                   "admin",
		Password:                   "super-secret-password",
		SSHPrivateKey:              "-----BEGIN RSA PRIVATE KEY-----",
		BearerToken:                "eyJhbGciOiJIUzI1NiJ9",
		TLSClientCertData:          "cert-data",
		TLSClientCertKey:           "cert-key",
		GCPServiceAccountKey:       "gcp-key",
		GithubAppPrivateKey:        "github-app-key",
		Insecure:                   true,
		EnableLFS:                  true,
		EnableOCI:                  true,
		Proxy:                      "http://proxy:8080",
		NoProxy:                    "localhost",
		Project:                    "default",
		ForceHttpBasicAuth:         true,
		InheritedCreds:             true,
		GithubAppId:                12345,
		GithubAppInstallationId:    67890,
		GitHubAppEnterpriseBaseURL: "https://ghe.example.com/api/v3",
		UseAzureWorkloadIdentity:   true,
		Depth:                      1,
	}

	sanitized := repo.Sanitized()

	// Non-sensitive fields must be preserved
	assert.Equal(t, repo.Repo, sanitized.Repo)
	assert.Equal(t, repo.Type, sanitized.Type)
	assert.Equal(t, repo.Name, sanitized.Name)
	assert.True(t, sanitized.Insecure)
	assert.Equal(t, repo.EnableLFS, sanitized.EnableLFS)
	assert.Equal(t, repo.EnableOCI, sanitized.EnableOCI)
	assert.Equal(t, repo.Proxy, sanitized.Proxy)
	assert.Equal(t, repo.NoProxy, sanitized.NoProxy)
	assert.Equal(t, repo.Project, sanitized.Project)
	assert.Equal(t, repo.ForceHttpBasicAuth, sanitized.ForceHttpBasicAuth)
	assert.Equal(t, repo.InheritedCreds, sanitized.InheritedCreds)
	assert.Equal(t, repo.GithubAppId, sanitized.GithubAppId)
	assert.Equal(t, repo.GithubAppInstallationId, sanitized.GithubAppInstallationId)
	assert.Equal(t, repo.GitHubAppEnterpriseBaseURL, sanitized.GitHubAppEnterpriseBaseURL)
	assert.Equal(t, repo.UseAzureWorkloadIdentity, sanitized.UseAzureWorkloadIdentity)
	assert.Equal(t, repo.Depth, sanitized.Depth)

	// Sensitive fields must be stripped
	assert.Empty(t, sanitized.Username)
	assert.Empty(t, sanitized.Password)
	assert.Empty(t, sanitized.SSHPrivateKey)
	assert.Empty(t, sanitized.BearerToken)
	assert.Empty(t, sanitized.TLSClientCertData)
	assert.Empty(t, sanitized.TLSClientCertKey)
	assert.Empty(t, sanitized.GCPServiceAccountKey)
	assert.Empty(t, sanitized.GithubAppPrivateKey)
}

func TestSanitizedRepositoryPreservesDepthZero(t *testing.T) {
	// Depth of 0 means full clone; verify it's preserved (zero value)
	repo := &Repository{
		Repo:  "https://github.com/argoproj/argo-cd.git",
		Depth: 0,
	}

	sanitized := repo.Sanitized()
	assert.Equal(t, int64(0), sanitized.Depth)
}

func TestGetGitCreds_GitHubApp_OrgExtractionFails(t *testing.T) {
	// This test verifies that when the organization cannot be extracted from the repo URL,
	// the credentials are still created but will provide a clear error when used.
	repo := &Repository{
		Repo:                "invalid-url-format",
		GithubAppPrivateKey: "github-key",
		GithubAppId:         123,
		// GithubAppInstallationId is 0 (not set), triggering auto-discovery
	}

	creds := repo.GetGitCreds(nil)

	// We should get GitHubAppCreds
	ghAppCreds, isGitHubAppCreds := creds.(git.GitHubAppCreds)
	require.True(t, isGitHubAppCreds, "expected GitHubAppCreds, got %T", creds)

	// When we try to use these credentials, we should get a clear error about org extraction failure
	_, _, err := ghAppCreds.Environ()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract organization")
	assert.Contains(t, err.Error(), "invalid-url-format")
}
