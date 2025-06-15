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
