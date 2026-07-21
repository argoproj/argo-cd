package v2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/identity/mocks"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
	repomocks "github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository/mocks"
)

func TestGetServiceAccountName(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		expected    string
	}{
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
			result, err := getServiceAccountName(tt.projectName)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// withPodToken points the pod service account token path at a file containing
// a JWT with the given subject for the duration of the test, resetting the
// cached name so the fixture is actually read.
func withPodToken(t *testing.T, subject string) {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": subject,
	}).SignedString([]byte("test-secret"))
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(token+"\n"), 0o600))

	orig := serviceAccountTokenPath
	serviceAccountTokenPath = path
	cachedPodSAName.Store(nil)
	t.Cleanup(func() {
		serviceAccountTokenPath = orig
		cachedPodSAName.Store(nil)
	})
}

func TestGetServiceAccountName_EmptyProjectUsesPodSA(t *testing.T) {
	withPodToken(t, "system:serviceaccount:argocd:argocd-repo-server")

	result, err := getServiceAccountName("")
	require.NoError(t, err)
	assert.Equal(t, "argocd-repo-server", result)

	// The name is cached after the first successful read: making the token
	// unreadable must not affect subsequent lookups.
	serviceAccountTokenPath = filepath.Join(t.TempDir(), "missing")
	result, err = getServiceAccountName("")
	require.NoError(t, err)
	assert.Equal(t, "argocd-repo-server", result)
}

func TestGetServiceAccountName_EmptyProjectErrors(t *testing.T) {
	t.Run("token not readable", func(t *testing.T) {
		orig := serviceAccountTokenPath
		serviceAccountTokenPath = filepath.Join(t.TempDir(), "missing")
		cachedPodSAName.Store(nil)
		t.Cleanup(func() { serviceAccountTokenPath = orig })

		_, err := getServiceAccountName("")
		require.ErrorContains(t, err, "pod service account token could not be read")
	})

	t.Run("non-serviceaccount subject", func(t *testing.T) {
		withPodToken(t, "some-user")

		_, err := getServiceAccountName("")
		require.ErrorContains(t, err, "unexpected subject")
	})

	t.Run("subject missing name", func(t *testing.T) {
		withPodToken(t, "system:serviceaccount:argocd:")

		_, err := getServiceAccountName("")
		require.ErrorContains(t, err, "unexpected subject")
	})
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
		wantError bool
	}{
		{
			name:      "k8s provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "", WorkloadIdentityProvider: "k8s", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantError: false,
		},
		{
			name:      "aws provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/repo", WorkloadIdentityProvider: "aws", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantError: false,
		},
		{
			name:      "gcp provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "oci://us-docker.pkg.dev/project/repo", WorkloadIdentityProvider: "gcp", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantError: false,
		},
		{
			name:      "azure provider - SA exists",
			repo:      &v1alpha1.Repository{Repo: "oci://myregistry.azurecr.io/repo", WorkloadIdentityProvider: "azure", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
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
			wantError: false,
		},
		{
			// Pod Identity does not use per-project SAs at all — construction must
			// succeed even when no SA is present in the cluster.
			name:      "aws provider - SA does not exist (Pod Identity friendly)",
			repo:      &v1alpha1.Repository{Repo: "oci://123456789012.dkr.ecr.us-west-2.amazonaws.com/repo", WorkloadIdentityProvider: "aws", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  false,
			wantError: false,
		},
		{
			name:      "unknown provider - returns error",
			repo:      &v1alpha1.Repository{WorkloadIdentityProvider: "unknown", Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantError: true,
		},
		{
			name:      "nil repository - returns error",
			repo:      nil,
			createSA:  false,
			wantError: true,
		},
		{
			name:      "empty provider - returns error",
			repo:      &v1alpha1.Repository{Project: "default"},
			saName:    "argocd-project-default",
			createSA:  true,
			wantError: true,
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
				assert.Nil(t, provider)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, provider)
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

func TestResolveCredentials_NilProvider(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := newTestResolver(clientset, "argocd")

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
	resolver := newTestResolver(clientset, "argocd")

	provider := mocks.NewProvider(t)
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
	resolver := newTestResolver(clientset, "argocd")

	provider := mocks.NewProvider(t)
	repoAuth := repository.NewHTTPTemplateAuthenticator()
	_, err := resolver.ResolveCredentials(context.Background(), provider, repoAuth, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository is required")
}

func TestResolveCredentials_ProviderTokenError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	resolver := newTestResolver(clientset, "argocd")

	provider := mocks.NewProvider(t)
	provider.EXPECT().GetToken(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("token request denied"))

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
	resolver := newTestResolver(clientset, "argocd")

	provider := mocks.NewProvider(t)
	provider.EXPECT().GetToken(mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Token{
			Type:  repository.TokenTypeBearer,
			Token: "test-token",
		}, nil)

	repoAuth := repomocks.NewAuthenticator(t)
	repoAuth.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("authentication failed"))

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
	resolver := newTestResolver(clientset, "argocd")

	provider := mocks.NewProvider(t)
	provider.EXPECT().GetToken(mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Token{
			Type:  repository.TokenTypeBearer,
			Token: "test-token",
		}, nil)

	repoAuth := repomocks.NewAuthenticator(t)
	repoAuth.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Credentials{
			Username: "user",
			Password: "pass",
		}, nil)

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
	resolver := newTestResolver(clientset, "argocd")

	provider := mocks.NewProvider(t)
	provider.EXPECT().GetToken(mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Token{
			Type:  repository.TokenTypeBearer,
			Token: "test-token",
		}, nil)

	var capturedConfig *repository.Config
	repoAuth := repomocks.NewAuthenticator(t)
	repoAuth.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *repository.Token, _ string, config *repository.Config) {
			capturedConfig = config
		}).
		Return(&repository.Credentials{Username: "user", Password: "pass"}, nil)

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
	resolver := newTestResolver(clientset, "argocd")

	var capturedAudience, capturedTokenURL string
	provider := mocks.NewProvider(t)
	provider.EXPECT().GetToken(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, audience string, tokenURL string) {
			capturedAudience = audience
			capturedTokenURL = tokenURL
		}).
		Return(&repository.Token{
			Type:  repository.TokenTypeBearer,
			Token: "test-token",
		}, nil)

	repoAuth := repomocks.NewAuthenticator(t)
	repoAuth.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.Credentials{Username: "user", Password: "pass"}, nil)

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
