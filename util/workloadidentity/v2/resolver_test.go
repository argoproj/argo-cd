package v2

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

func TestGetServiceAccountName(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		expected    string
	}{
		{
			name:        "empty project returns global SA",
			projectName: "",
			expected:    "argocd-global",
		},
		{
			name:        "default project",
			projectName: "default",
			expected:    "argocd-project-default",
		},
		{
			name:        "custom project",
			projectName: "my-project",
			expected:    "argocd-project-my-project",
		},
		{
			name:        "project with hyphens",
			projectName: "team-a-prod",
			expected:    "argocd-project-team-a-prod",
		},
		{
			name:        "project with numbers",
			projectName: "project123",
			expected:    "argocd-project-project123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getServiceAccountName(tt.projectName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewResolver(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	namespace := "argocd"

	resolver := NewResolver(clientset, namespace)

	require.NotNil(t, resolver)
	assert.NotNil(t, resolver.serviceAccounts)
}

func TestNewIdentityProvider(t *testing.T) {
	namespace := "argocd"

	tests := []struct {
		name      string
		repo      *v1alpha1.Repository
		saName    string
		createSA  bool
		wantNil   bool
		wantError bool
	}{
		{
			name:      "k8s provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "", WorkloadIdentityProvider: "k8s", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantNil:   false,
			wantError: false,
		},
		{
			name:      "aws provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/repo", WorkloadIdentityProvider: "aws", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantNil:   false,
			wantError: false,
		},
		{
			name:      "gcp provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "oci://us-docker.pkg.dev/project/repo", WorkloadIdentityProvider: "gcp", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantNil:   false,
			wantError: false,
		},
		{
			name:      "azure provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "oci://myregistry.azurecr.io/repo", WorkloadIdentityProvider: "azure", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantNil:   false,
			wantError: false,
		},
		{
			// Construction is lazy: NewIdentityProvider no longer fetches the SA up
			// front, so a missing argocd-project-<project> SA is not an error here.
			// The SA fetch happens inside provider.GetToken for paths that need it
			// (IRSA, GCP, Azure, K8s); AWS Pod Identity skips it entirely.
			name:      "k8s provider - SA does not exist (lazy fetch)",
			repo:      &v1alpha1.Repository{Repo: "", WorkloadIdentityProvider: "k8s", Project: "nonexistent"},
			saName:    "argocd-project-nonexistent",
			createSA:  false,
			wantNil:   false,
			wantError: false,
		},
		{
			// Pod Identity does not use per-project SAs at all — construction must
			// succeed even when no SA is present in the cluster.
			name:      "aws provider - SA does not exist (Pod Identity friendly)",
			repo:      &v1alpha1.Repository{Repo: "oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/repo", WorkloadIdentityProvider: "aws", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  false,
			wantNil:   false,
			wantError: false,
		},
		{
			name:      "unknown provider - returns nil",
			repo:      &v1alpha1.Repository{WorkloadIdentityProvider: "unknown", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantNil:   true,
			wantError: false,
		},
		{
			name:      "empty provider - returns nil",
			repo:      &v1alpha1.Repository{Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantNil:   true,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var clientset *fake.Clientset
			if tt.createSA {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.saName,
						Namespace: namespace,
					},
				}
				clientset = fake.NewSimpleClientset(sa)
			} else {
				clientset = fake.NewSimpleClientset()
			}

			provider, err := NewIdentityProvider(tt.repo, clientset, namespace)

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, provider)
			} else {
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestNewAuthenticator(t *testing.T) {
	tests := []struct {
		name          string
		authenticator string
		wantNil       bool
	}{
		{name: "ecr authenticator", authenticator: "ecr", wantNil: false},
		{name: "passthrough authenticator", authenticator: "passthrough", wantNil: false},
		{name: "acr authenticator", authenticator: "acr", wantNil: false},
		{name: "http authenticator", authenticator: "http", wantNil: false},
		{name: "codecommit authenticator", authenticator: "codecommit", wantNil: true},
		{name: "unknown authenticator", authenticator: "unknown", wantNil: true},
		{name: "empty authenticator", authenticator: "", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewAuthenticator(tt.authenticator)
			if tt.wantNil {
				assert.Nil(t, auth)
			} else {
				assert.NotNil(t, auth)
			}
		})
	}
}

// mockProvider is a test mock for identity.Provider
type mockProvider struct {
	getTokenFunc                      func(ctx context.Context, audience, tokenURL string) (*repository.Token, error)
	defaultRepositoryAuthenticatorVal repository.Authenticator
}

func (m *mockProvider) GetToken(ctx context.Context, audience, tokenURL string) (*repository.Token, error) {
	if m.getTokenFunc != nil {
		return m.getTokenFunc(ctx, audience, tokenURL)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProvider) DefaultRepositoryAuthenticator() repository.Authenticator {
	return m.defaultRepositoryAuthenticatorVal
}

// mockAuthenticator is a test mock for repository.Authenticator
type mockAuthenticator struct {
	authenticateFunc func(ctx context.Context, token *repository.Token, repoURL string, config *repository.Config) (*repository.Credentials, error)
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, token *repository.Token, repoURL string, config *repository.Config) (*repository.Credentials, error) {
	if m.authenticateFunc != nil {
		return m.authenticateFunc(ctx, token, repoURL, config)
	}
	return nil, errors.New("not implemented")
}

func TestResolveCredentials_NilProvider(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	repo := &v1alpha1.Repository{
		Repo:                     "https://example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "aws",
	}
	repoAuth := repository.NewECRAuthenticator()
	_, err := resolver.ResolveCredentials(context.Background(), nil, repoAuth, repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity provider is required")
}

func TestResolveCredentials_NilAuthenticator(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	provider := &mockProvider{}
	repo := &v1alpha1.Repository{
		Repo:                     "https://example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "k8s",
	}
	_, err := resolver.ResolveCredentials(context.Background(), provider, nil, repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository authenticator is required")
}

func TestResolveCredentials_NilRepo(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	provider := &mockProvider{}
	repoAuth := repository.NewHTTPTemplateAuthenticator()
	_, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository is required")
}

func TestResolveCredentials_ProviderTokenError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	provider := &mockProvider{
		getTokenFunc: func(ctx context.Context, audience, tokenURL string) (*repository.Token, error) {
			return nil, errors.New("token request denied")
		},
	}

	repo := &v1alpha1.Repository{
		Repo:                     "https://example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "aws",
	}
	repoAuth := repository.NewECRAuthenticator()

	_, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity provider failed")
	assert.Contains(t, err.Error(), "token request denied")
}

func TestResolveCredentials_AuthenticatorError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	provider := &mockProvider{
		getTokenFunc: func(ctx context.Context, audience, tokenURL string) (*repository.Token, error) {
			return &repository.Token{
				Type:  repository.TokenTypeBearer,
				Token: "test-token",
			}, nil
		},
	}

	repoAuth := &mockAuthenticator{
		authenticateFunc: func(ctx context.Context, token *repository.Token, repoURL string, config *repository.Config) (*repository.Credentials, error) {
			return nil, errors.New("authentication failed")
		},
	}

	repo := &v1alpha1.Repository{
		Repo:                     "https://example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "aws",
	}

	_, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestResolveCredentials_Success(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	provider := &mockProvider{
		getTokenFunc: func(ctx context.Context, audience, tokenURL string) (*repository.Token, error) {
			return &repository.Token{
				Type:  repository.TokenTypeBearer,
				Token: "test-token",
			}, nil
		},
	}

	repoAuth := &mockAuthenticator{
		authenticateFunc: func(ctx context.Context, token *repository.Token, repoURL string, config *repository.Config) (*repository.Credentials, error) {
			return &repository.Credentials{
				Username: "user",
				Password: "pass",
			}, nil
		},
	}

	repo := &v1alpha1.Repository{
		Repo:                     "https://example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "aws",
	}

	creds, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.NoError(t, err)
	assert.Equal(t, "user", creds.Username)
	assert.Equal(t, "pass", creds.Password)
}

func TestResolveCredentials_PassesConfigToAuthenticator(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	provider := &mockProvider{
		getTokenFunc: func(ctx context.Context, audience, tokenURL string) (*repository.Token, error) {
			return &repository.Token{
				Type:  repository.TokenTypeBearer,
				Token: "test-token",
			}, nil
		},
	}

	var capturedConfig *repository.Config
	repoAuth := &mockAuthenticator{
		authenticateFunc: func(ctx context.Context, token *repository.Token, repoURL string, config *repository.Config) (*repository.Credentials, error) {
			capturedConfig = config
			return &repository.Credentials{Username: "user", Password: "pass"}, nil
		},
	}

	repo := &v1alpha1.Repository{
		Repo:                               "https://example.com",
		Project:                            "default",
		WorkloadIdentityProvider:           "aws",
		WorkloadIdentityUsername:           "custom-user",
		Insecure:                           true,
		WorkloadIdentityAuthHost:           "auth.example.com",
		WorkloadIdentityMethod:             "POST",
		WorkloadIdentityPathTemplate:       "/api/token",
		WorkloadIdentityBodyTemplate:       `{"token": "{{.Token}}"}`,
		WorkloadIdentityAuthType:           "bearer",
		WorkloadIdentityParams:             map[string]string{"param1": "value1"},
		WorkloadIdentityResponseTokenField: "access_token",
	}

	_, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.NoError(t, err)

	require.NotNil(t, capturedConfig)
	assert.Equal(t, "custom-user", capturedConfig.Username)
	assert.True(t, capturedConfig.Insecure)
	assert.Equal(t, "auth.example.com", capturedConfig.AuthHost)
	assert.Equal(t, "POST", capturedConfig.Method)
	assert.Equal(t, "/api/token", capturedConfig.PathTemplate)
	assert.JSONEq(t, `{"token": "{{.Token}}"}`, capturedConfig.BodyTemplate)
	assert.Equal(t, "bearer", capturedConfig.AuthType)
	assert.Equal(t, map[string]string{"param1": "value1"}, capturedConfig.Params)
	assert.Equal(t, "access_token", capturedConfig.ResponseTokenField)
}

func TestResolveCredentials_PassesAudienceAndTokenURL(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := NewResolver(clientset, "argocd")

	var capturedAudience, capturedTokenURL string
	provider := &mockProvider{
		getTokenFunc: func(ctx context.Context, audience, tokenURL string) (*repository.Token, error) {
			capturedAudience = audience
			capturedTokenURL = tokenURL
			return &repository.Token{
				Type:  repository.TokenTypeBearer,
				Token: "test-token",
			}, nil
		},
	}

	repoAuth := &mockAuthenticator{
		authenticateFunc: func(ctx context.Context, token *repository.Token, repoURL string, config *repository.Config) (*repository.Credentials, error) {
			return &repository.Credentials{Username: "user", Password: "pass"}, nil
		},
	}

	repo := &v1alpha1.Repository{
		Repo:                     "https://example.com",
		Project:                  "default",
		WorkloadIdentityProvider: "aws",
		WorkloadIdentityAudience: "custom-audience",
		WorkloadIdentityTokenURL: "https://custom-sts.example.com",
	}

	_, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, repo)
	require.NoError(t, err)

	assert.Equal(t, "custom-audience", capturedAudience)
	assert.Equal(t, "https://custom-sts.example.com", capturedTokenURL)
}
