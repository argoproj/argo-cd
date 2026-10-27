package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/go-playground/webhooks/v6/azuredevops"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db/mocks"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	testADORepoURL   = "https://dev.azure.com/my-org/my-project/_git/my-repo"
	testADORepoID    = "ba2967cc-02c2-414c-8d10-1b99197cbaa6"
	testADOShaBefore = "fa51eeb1e50b98293ce281e6d5492b9decae613b"
	testADOShaAfter  = "298a79aa1552799a70718a0ee914d153d5a1a76b"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newADOWebhookSettings() *settings.ArgoCDSettings {
	return &settings.ArgoCDSettings{
		WebhookAzureDevOpsUsername: "webhook-user",
		WebhookAzureDevOpsPassword: "webhook-pass",
	}
}

func newMockHandlerForADO(t *testing.T, mockDB *mocks.ArgoDB, argoSettings *settings.ArgoCDSettings, objects ...runtime.Object) *ArgoCDWebhookHandler {
	t.Helper()
	h := newMockHandler(nil, []string{}, int64(50)*1024*1024, mockDB, argoSettings, objects...)
	t.Cleanup(h.Shutdown)
	return h
}

func newMockDBWithADORepository() *mocks.ArgoDB {
	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{{
		Repo:     testADORepoURL,
		Username: "test-user",
		Password: "test-pat",
	}}, nil)
	return mockDB
}

func loadADOPushEvent(t *testing.T) azuredevops.GitPushEvent {
	t.Helper()
	eventJSON, err := os.ReadFile("testdata/azuredevops-git-push-event.json")
	require.NoError(t, err)
	var payload azuredevops.GitPushEvent
	require.NoError(t, json.Unmarshal(eventJSON, &payload))
	return payload
}

func newADODiffsTestClient(t *testing.T, handler func(req *http.Request) (int, adoDiffsResponse)) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		statusCode, diffsResp := handler(req)
		respBody, err := json.Marshal(diffsResp)
		require.NoError(t, err)
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(respBody)),
			Request:    req,
		}, nil
	})}
}

func requireADODiffsRequest(t *testing.T, req *http.Request, expectedPath, expectedShaBefore, expectedShaAfter string) {
	t.Helper()
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, expectedPath, req.URL.Path)
	query := req.URL.Query()
	require.Equal(t, expectedShaBefore, query.Get("baseVersion"))
	require.Equal(t, "commit", query.Get("baseVersionType"))
	require.Equal(t, expectedShaAfter, query.Get("targetVersion"))
	require.Equal(t, "commit", query.Get("targetVersionType"))
	require.Equal(t, strconv.Itoa(adoDiffsMaxPageSize), query.Get("$top"))
	require.Equal(t, adoDiffsAPIVersion, query.Get("api-version"))
	username, password, ok := req.BasicAuth()
	require.True(t, ok)
	require.Equal(t, "test-user", username)
	require.Equal(t, "test-pat", password)
}

func adoDiffsTestResponse() adoDiffsResponse {
	return adoDiffsResponse{
		AllChangesIncluded: true,
		Changes: []adoDiffChange{
			{
				Item:       adoDiffItem{Path: "/apps/my-app/values.yaml", IsFolder: false},
				ChangeType: "edit",
			},
			{
				Item:       adoDiffItem{Path: "/apps/my-app/deployment.yaml", IsFolder: false},
				ChangeType: "add",
			},
			{
				// Folder entries should be excluded from the changed files list.
				Item:       adoDiffItem{Path: "/apps/my-app", IsFolder: true},
				ChangeType: "edit",
			},
		},
	}
}

func Test_affectedRevisionInfo_azuredevops_changed_files(t *testing.T) {
	client := newADODiffsTestClient(t, func(req *http.Request) (int, adoDiffsResponse) {
		requireADODiffsRequest(t, req, "/my-org/my-project/_apis/git/repositories/"+testADORepoID+"/diffs/commits", testADOShaBefore, testADOShaAfter)
		return http.StatusOK, adoDiffsTestResponse()
	})

	h := newMockHandlerForADO(t, newMockDBWithADORepository(), newADOWebhookSettings())
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		return client
	}
	_, _, _, _, changedFiles := h.affectedRevisionInfo(t.Context(), loadADOPushEvent(t))
	require.Equal(t, []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"}, changedFiles)
}

func Test_affectedRevisionInfo_azuredevops_changed_files_via_creds_template(t *testing.T) {
	client := newADODiffsTestClient(t, func(req *http.Request) (int, adoDiffsResponse) {
		requireADODiffsRequest(t, req, "/my-org/my-project/_apis/git/repositories/"+testADORepoID+"/diffs/commits", testADOShaBefore, testADOShaAfter)
		return http.StatusOK, adoDiffsTestResponse()
	})

	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil)
	mockDB.EXPECT().GetRepositoryCredentials(mock.Anything, testADORepoURL).Return(&v1alpha1.RepoCreds{
		URL:      "https://dev.azure.com/my-org",
		Username: "test-user",
		Password: "test-pat",
		Proxy:    "http://proxy.example.com:8080",
		NoProxy:  "localhost",
	}, nil)
	h := newMockHandlerForADO(t, mockDB, newADOWebhookSettings())
	h.adoHTTPClientFactory = func(repo *v1alpha1.Repository) *http.Client {
		require.Equal(t, testADORepoURL, repo.Repo)
		require.Equal(t, "http://proxy.example.com:8080", repo.Proxy)
		require.Equal(t, "localhost", repo.NoProxy)
		return client
	}
	_, _, _, _, changedFiles := h.affectedRevisionInfo(t.Context(), loadADOPushEvent(t))
	require.Equal(t, []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"}, changedFiles)
}

func Test_affectedRevisionInfo_azuredevops_without_webhook_auth_skips_changed_files_lookup(t *testing.T) {
	h := newMockHandlerForADO(t, newMockDBWithADORepository(), &settings.ArgoCDSettings{})
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		t.Fatal("ADO HTTP client should not be created when Azure DevOps webhook authentication is not configured")
		return nil
	}
	_, _, _, _, changedFiles := h.affectedRevisionInfo(t.Context(), loadADOPushEvent(t))
	require.Empty(t, changedFiles)
}

func Test_affectedRevisionInfo_azuredevops_multiple_ref_updates(t *testing.T) {
	client := newADODiffsTestClient(t, func(req *http.Request) (int, adoDiffsResponse) {
		// Only the first refUpdate's SHAs should be used.
		requireADODiffsRequest(t, req, "/my-org/my-project/_apis/git/repositories/"+testADORepoID+"/diffs/commits", testADOShaBefore, testADOShaAfter)
		return http.StatusOK, adoDiffsTestResponse()
	})

	payload := loadADOPushEvent(t)
	payload.Resource.RefUpdates = append(payload.Resource.RefUpdates, azuredevops.RefUpdate{
		Name:        "refs/heads/other-branch",
		OldObjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		NewObjectID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})

	h := newMockHandlerForADO(t, newMockDBWithADORepository(), newADOWebhookSettings())
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		return client
	}
	_, _, _, _, changedFiles := h.affectedRevisionInfo(t.Context(), payload)
	require.Equal(t, []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"}, changedFiles)
}

func Test_affectedRevisionInfo_azuredevops_no_ref_updates_skips_changed_files_lookup(t *testing.T) {
	payload := loadADOPushEvent(t)
	payload.Resource.RefUpdates = nil

	h := newMockHandlerForADO(t, newMockDBWithADORepository(), newADOWebhookSettings())
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		t.Fatal("ADO HTTP client should not be created when the webhook has no ref updates")
		return nil
	}
	_, _, _, _, changedFiles := h.affectedRevisionInfo(t.Context(), payload)
	require.Empty(t, changedFiles)
}

func TestAzureDevOpsWebhookAuthConfigured(t *testing.T) {
	tests := []struct {
		name     string
		settings *settings.ArgoCDSettings
		expected bool
	}{
		{
			name:     "no username or password",
			settings: &settings.ArgoCDSettings{},
			expected: false,
		},
		{
			name:     "username only",
			settings: &settings.ArgoCDSettings{WebhookAzureDevOpsUsername: "webhook-user"},
			expected: true,
		},
		{
			name:     "password only",
			settings: &settings.ArgoCDSettings{WebhookAzureDevOpsPassword: "webhook-pass"},
			expected: true,
		},
		{
			name:     "username and password",
			settings: newADOWebhookSettings(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &ArgoCDWebhookHandler{settings: tt.settings}
			require.Equal(t, tt.expected, h.azureDevOpsWebhookAuthConfigured())
		})
	}
}

func TestLookupAndFetchADOChangedFiles_SkipsNonDiffableRange(t *testing.T) {
	tests := []struct {
		name      string
		shaBefore string
		shaAfter  string
	}{
		{
			name:      "branch deletion (shaAfter is null SHA)",
			shaBefore: testADOShaBefore,
			shaAfter:  adoNullSHA,
		},
		{
			name:      "branch creation (shaBefore is null SHA)",
			shaBefore: adoNullSHA,
			shaAfter:  testADOShaAfter,
		},
		{
			name:      "empty commit range (no refUpdates)",
			shaBefore: "",
			shaAfter:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newMockHandlerForADO(t, &mocks.ArgoDB{}, newADOWebhookSettings())
			h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
				t.Fatal("ADO HTTP client should not be created for non-diffable commit range")
				return nil
			}
			changedFiles := h.lookupAndFetchADOChangedFiles(t.Context(), testADORepoURL, testADORepoID, tt.shaBefore, tt.shaAfter)
			require.Empty(t, changedFiles)
		})
	}
}

func TestLookupAndFetchADOChangedFiles_NoRepositoryConfigured(t *testing.T) {
	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{}, nil)
	mockDB.EXPECT().GetRepositoryCredentials(mock.Anything, testADORepoURL).Return(nil, nil)
	h := newMockHandlerForADO(t, mockDB, newADOWebhookSettings())
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		t.Fatal("ADO HTTP client should not be created for a repository that is not configured in Argo CD")
		return nil
	}

	changedFiles := h.lookupAndFetchADOChangedFiles(t.Context(), testADORepoURL, testADORepoID, testADOShaBefore, testADOShaAfter)
	require.Empty(t, changedFiles)
}

func TestLookupAndFetchADOChangedFiles_RepositoryLookupError(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return(nil, errors.New("list failed"))
	h := newMockHandlerForADO(t, mockDB, newADOWebhookSettings())
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		t.Fatal("ADO HTTP client should not be created when repository lookup fails")
		return nil
	}

	changedFiles := h.lookupAndFetchADOChangedFiles(t.Context(), testADORepoURL, testADORepoID, testADOShaBefore, testADOShaAfter)
	require.Empty(t, changedFiles)
	assertLogContainsSubstr(t, hook, "error finding repository for Azure DevOps webhook URL")
}

func TestLookupAndFetchADOChangedFiles_FetchError(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	h := newMockHandlerForADO(t, newMockDBWithADORepository(), newADOWebhookSettings())
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("ado request failed")
		})}
	}

	changedFiles := h.lookupAndFetchADOChangedFiles(t.Context(), testADORepoURL, testADORepoID, testADOShaBefore, testADOShaAfter)
	require.Empty(t, changedFiles)
	assertLogContainsSubstr(t, hook, "error fetching changed files from Azure DevOps diffs API")
}

func TestLookupAndFetchADOChangedFiles_NoCredentials(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	mockDB := &mocks.ArgoDB{}
	mockDB.EXPECT().ListRepositories(mock.Anything).Return([]*v1alpha1.Repository{{Repo: testADORepoURL}}, nil)
	h := newMockHandlerForADO(t, mockDB, newADOWebhookSettings())
	h.adoHTTPClientFactory = func(*v1alpha1.Repository) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("ADO REST API should not be called without credentials")
			return nil, nil
		})}
	}

	changedFiles := h.lookupAndFetchADOChangedFiles(t.Context(), testADORepoURL, testADORepoID, testADOShaBefore, testADOShaAfter)
	require.Empty(t, changedFiles)
	assertLogContainsSubstr(t, hook, "no supported Azure DevOps REST API credentials configured for repository")
}

func TestParseADOBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		expectedURL string
		expectErr   bool
	}{
		{
			name:        "modern dev.azure.com URL",
			repoURL:     "https://dev.azure.com/myorg/myproject/_git/myrepo",
			expectedURL: "https://dev.azure.com/myorg/myproject",
		},
		{
			name:        "legacy visualstudio.com URL",
			repoURL:     "https://myorg.visualstudio.com/myproject/_git/myrepo",
			expectedURL: "https://myorg.visualstudio.com/myproject",
		},
		{
			name:      "legacy visualstudio.com URL missing project",
			repoURL:   "https://myorg.visualstudio.com/",
			expectErr: true,
		},
		{
			name:      "URL with only org segment",
			repoURL:   "https://dev.azure.com/myorg",
			expectErr: true,
		},
		{
			name:      "URL with empty project segment",
			repoURL:   "https://dev.azure.com/myorg/",
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseADOBaseURL(tt.repoURL)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedURL, result)
		})
	}
}

func TestFetchChangedFilesFromADO(t *testing.T) {
	const (
		testRepoURL        = "https://dev.azure.com/myorg/myproject/_git/myrepo"
		testShaBeforeLarge = "1111111111111111111111111111111111111111"
		testShaAfterLarge  = "2222222222222222222222222222222222222222"
	)

	client := newADODiffsTestClient(t, func(req *http.Request) (int, adoDiffsResponse) {
		switch req.URL.Path {
		case "/myorg/myproject/_apis/git/repositories/" + testADORepoID + "/diffs/commits":
			if req.URL.Query().Get("baseVersion") == testShaBeforeLarge {
				requireADODiffsRequest(t, req, req.URL.Path, testShaBeforeLarge, testShaAfterLarge)
				return http.StatusOK, adoDiffsResponse{AllChangesIncluded: false}
			}
			if req.URL.Query().Get("baseVersion") == testADOShaBefore {
				requireADODiffsRequest(t, req, req.URL.Path, testADOShaBefore, testADOShaAfter)
				return http.StatusOK, adoDiffsTestResponse()
			}
		case "/myproject/_apis/git/repositories/" + testADORepoID + "/diffs/commits":
			requireADODiffsRequest(t, req, req.URL.Path, testADOShaBefore, testADOShaAfter)
			return http.StatusOK, adoDiffsTestResponse()
		}
		return http.StatusNotFound, adoDiffsResponse{AllChangesIncluded: true}
	})

	tests := []struct {
		name          string
		repoURL       string
		shaBefore     string
		shaAfter      string
		expectedFiles []string
		expectErr     bool
	}{
		{
			name:          "returns changed files for valid diff",
			repoURL:       testRepoURL,
			shaBefore:     testADOShaBefore,
			shaAfter:      testADOShaAfter,
			expectedFiles: []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"},
		},
		{
			name:      "returns error when diff is truncated (allChangesIncluded=false)",
			repoURL:   testRepoURL,
			shaBefore: testShaBeforeLarge,
			shaAfter:  testShaAfterLarge,
			expectErr: true,
		},
		{
			name:      "returns error on non-200 status",
			repoURL:   testRepoURL,
			shaBefore: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			shaAfter:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			expectErr: true,
		},
		{
			name:          "returns changed files for legacy visualstudio.com URL",
			repoURL:       "https://myorg.visualstudio.com/myproject/_git/myrepo",
			shaBefore:     testADOShaBefore,
			shaAfter:      testADOShaAfter,
			expectedFiles: []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"},
		},
		{
			name:      "returns error for URL without org and project",
			repoURL:   "https://dev.azure.com",
			shaBefore: testADOShaBefore,
			shaAfter:  testADOShaAfter,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &v1alpha1.Repository{
				Repo:     tt.repoURL,
				Username: "test-user",
				Password: "test-pat",
			}
			changedFiles, err := fetchChangedFilesFromADO(t.Context(), client, repo, testADORepoID, tt.shaBefore, tt.shaAfter)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedFiles, changedFiles)
		})
	}
}

func TestFetchChangedFilesFromADO_NoCredentials(t *testing.T) {
	repo := &v1alpha1.Repository{
		Repo: "https://dev.azure.com/myorg/myproject/_git/myrepo",
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP client should not be called when no supported ADO REST API credentials are configured")
		return nil, nil
	})}

	changedFiles, err := fetchChangedFilesFromADO(t.Context(), client, repo, "some-repo-id", "sha-before", "sha-after")
	require.ErrorIs(t, err, errNoADOCredentials)
	require.Nil(t, changedFiles)
}

func TestFetchChangedFilesFromADO_BearerTokenAuth(t *testing.T) {
	client := newADODiffsTestClient(t, func(req *http.Request) (int, adoDiffsResponse) {
		require.Equal(t, "/myorg/myproject/_apis/git/repositories/"+testADORepoID+"/diffs/commits", req.URL.Path)
		require.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
		return http.StatusOK, adoDiffsTestResponse()
	})
	repo := &v1alpha1.Repository{
		Repo:        "https://dev.azure.com/myorg/myproject/_git/myrepo",
		BearerToken: "test-token",
	}

	changedFiles, err := fetchChangedFilesFromADO(t.Context(), client, repo, testADORepoID, testADOShaBefore, testADOShaAfter)
	require.NoError(t, err)
	require.Equal(t, []string{"apps/my-app/values.yaml", "apps/my-app/deployment.yaml"}, changedFiles)
}

func TestSetADOAuthHeader(t *testing.T) {
	tests := []struct {
		name        string
		repo        *v1alpha1.Repository
		wantApplied bool
		wantBasic   bool
		wantUser    string
		wantPass    string
		wantBearer  string
	}{
		{
			name:        "basic auth with username and password",
			repo:        &v1alpha1.Repository{Username: "test-user", Password: "test-pat"},
			wantApplied: true,
			wantBasic:   true,
			wantUser:    "test-user",
			wantPass:    "test-pat",
		},
		{
			name:        "personal access token without username",
			repo:        &v1alpha1.Repository{Password: "test-pat"},
			wantApplied: true,
			wantBasic:   true,
			wantPass:    "test-pat",
		},
		{
			name:        "bearer token only",
			repo:        &v1alpha1.Repository{BearerToken: "test-token"},
			wantApplied: true,
			wantBearer:  "Bearer test-token",
		},
		{
			name:        "bearer token takes precedence over basic auth",
			repo:        &v1alpha1.Repository{Username: "test-user", Password: "test-pat", BearerToken: "test-token"},
			wantApplied: true,
			wantBearer:  "Bearer test-token",
		},
		{
			name: "username without password",
			repo: &v1alpha1.Repository{Username: "test-user"},
		},
		{
			name: "no supported credentials",
			repo: &v1alpha1.Repository{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://dev.azure.com/my-org/my-project/_apis", http.NoBody)
			require.NoError(t, err)

			require.Equal(t, tt.wantApplied, setADOAuthHeader(req, tt.repo))

			user, pass, ok := req.BasicAuth()
			require.Equal(t, tt.wantBasic, ok)
			require.Equal(t, tt.wantUser, user)
			require.Equal(t, tt.wantPass, pass)
			if tt.wantBearer != "" {
				require.Equal(t, tt.wantBearer, req.Header.Get("Authorization"))
			} else if !tt.wantBasic {
				require.Empty(t, req.Header.Get("Authorization"))
			}
		})
	}
}

func TestNewADOHTTPClientUsesRepositoryTransportSettings(t *testing.T) {
	repo := &v1alpha1.Repository{
		Repo:     testADORepoURL,
		Username: "test-user",
		Password: "test-pat",
		Proxy:    "http://proxy.example.com:8080",
		NoProxy:  "localhost",
	}

	client := newADOHTTPClient(repo)
	require.Positive(t, client.Timeout)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://dev.azure.com/my-org/my-project/_apis", http.NoBody)
	require.NoError(t, err)
	proxyURL, err := transport.Proxy(req)
	require.NoError(t, err)
	require.Equal(t, "http://proxy.example.com:8080", proxyURL.String())

	noProxyReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://localhost/_apis", http.NoBody)
	require.NoError(t, err)
	proxyURL, err = transport.Proxy(noProxyReq)
	require.NoError(t, err)
	require.Nil(t, proxyURL)
}
