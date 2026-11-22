package identity

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

func TestAzureProvider_RepoTypeSelectsScopeAndAuthenticator(t *testing.T) {
	tests := []struct {
		name     string
		repo     v1alpha1.Repository
		wantGit  bool
		wantUser string
	}{
		{name: "explicit git", repo: v1alpha1.Repository{Repo: "https://dev.azure.com/org/p/_git/r", Type: "git"}, wantGit: true, wantUser: "oauth"},
		{name: "empty type defaults to git", repo: v1alpha1.Repository{Repo: "https://dev.azure.com/org/p/_git/r"}, wantGit: true, wantUser: "oauth"},
		{name: "empty type oci URL", repo: v1alpha1.Repository{Repo: "oci://myacr.azurecr.io/charts"}},
		{name: "helm", repo: v1alpha1.Repository{Repo: "myacr.azurecr.io/charts", Type: "helm", EnableOCI: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOSTNAME", "")
			var scope string
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.NoError(t, r.ParseForm())
				scope = r.PostForm.Get("scope")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"azure-token","expires_in":3600}`))
			}))
			defer tokenServer.Close()

			api := &fakeAPIServer{
				reviewStatus:  http.StatusUnauthorized,
				saAnnotations: map[string]string{AnnotationAzureClientID: "client", AnnotationAzureTenantID: "tenant"},
			}
			p := NewAzureProvider(&tt.repo, NewK8sProvider(api.clientset(t), "argocd", "argocd-project-default"))

			token, err := p.GetToken(t.Context(), "", tokenServer.URL)
			require.NoError(t, err)
			assert.Equal(t, tt.wantUser, token.Username)
			if tt.wantGit {
				assert.Equal(t, azureDevOpsScope, scope)
				assert.IsType(t, &repository.PassthroughAuthenticator{}, p.DefaultRepositoryAuthenticator())
			} else {
				assert.Equal(t, "https://management.azure.com/.default", scope)
				assert.IsType(t, &repository.ACRAuthenticator{}, p.DefaultRepositoryAuthenticator())
			}
		})
	}
}

type routeTransport map[string]string

func (rt routeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := rt[req.URL.String()]
	status := http.StatusOK
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestGCPProvider_MetadataIdentityIsTarget(t *testing.T) {
	const target = "repo-reader@proj.iam.gserviceaccount.com"
	tests := []struct {
		name      string
		podEmail  string
		wantToken string
	}{
		// IAM Credentials won't mint for the caller's own account.
		{name: "pod runs as target", podEmail: target, wantToken: "pod-token"},
		{name: "pod runs as another account", podEmail: "node@proj.iam.gserviceaccount.com", wantToken: "impersonated-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := httpClient
			t.Cleanup(func() { httpClient = orig })
			httpClient = &http.Client{Transport: routeTransport{
				GCPMetadataTokenURL: `{"access_token":"pod-token","expires_in":3600}`,
				GCPMetadataEmailURL: tt.podEmail,
				strings.Replace(DefaultGCPIAMCredentialsURL, "%s", target, 1): `{"accessToken":"impersonated-token","expireTime":"2099-01-01T00:00:00Z"}`,
			}}

			api := &fakeAPIServer{saAnnotations: map[string]string{AnnotationGCPSA: target}}
			p := NewGCPProvider(NewK8sProvider(api.clientset(t), "argocd", "argocd-project-default"))

			token, err := p.GetToken(t.Context(), "", "")
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token.Token)
			assert.NotNil(t, token.ExpiresAt)
		})
	}
}

func TestBoundedSessionName(t *testing.T) {
	assert.Equal(t, "argocd-proj-team-a", boundedSessionName("argocd-proj-", "team-a"))

	long := strings.Repeat("p", 53) // 12-char prefix + 53 = 65
	got := boundedSessionName("argocd-proj-", long)
	assert.Len(t, got, 64)
	assert.True(t, strings.HasPrefix(got, "argocd-proj-ppp"))
	assert.NotEqual(t, got, boundedSessionName("argocd-proj-", long+"x"), "distinct names must stay distinct")
}
