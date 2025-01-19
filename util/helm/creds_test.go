package helm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoutils "github.com/argoproj/argo-cd/v3/util"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/mocks"
)

func TestWorkLoadIdentityUserNameShouldBeEmptyGuid(t *testing.T) {
	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io/charts", "", nil, nil, false, workloadIdentityMock)
	username := creds.GetUsername()

	assert.Equal(t, workloadidentity.EmptyGuid, username, "The username for azure workload identity is not empty Guid")
}

func TestGetAccessTokenShouldReturnTokenFromCacheIfPresent(t *testing.T) {
	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io/charts", "", nil, nil, false, workloadIdentityMock)

	cacheKey, err := argoutils.GenerateCacheKey("accesstoken-%s", "contoso.azurecr.io")
	require.NoError(t, err, "Error generating cache key")

	// Store the token in the cache
	storeAzureToken(cacheKey, "testToken", time.Hour)

	// Retrieve the token from the cache
	token, err := creds.GetAccessToken()
	require.NoError(t, err, "Error getting access token")
	assert.Equal(t, "testToken", token, "The retrieved token should match the stored token")
}

func TestGetPasswordShouldReturnTokenFromCacheIfPresent(t *testing.T) {
	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io/charts", "", nil, nil, false, workloadIdentityMock)

	cacheKey, err := argoutils.GenerateCacheKey("accesstoken-%s", "contoso.azurecr.io")
	require.NoError(t, err, "Error generating cache key")

	// Store the token in the cache
	storeAzureToken(cacheKey, "testToken", time.Hour)

	// Retrieve the token from the cache
	token, err := creds.GetPassword()
	require.NoError(t, err, "Error getting access token")
	assert.Equal(t, "testToken", token, "The retrieved token should match the stored token")
}

func TestGetPasswordShouldGenerateTokenIfNotPresentInCache(t *testing.T) {
	mockUrl := ""
	serverUrl := func() string {
		return mockUrl
	}

	// Mock the server to return a successful response
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm="%s",service="%s"`, serverUrl(), serverUrl()[8:]))
			w.WriteHeader(http.StatusUnauthorized)

		case "/oauth2/exchange":
			response := `{"refresh_token":"newRefreshToken"}`
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(response))
			require.NoError(t, err)
		}
	}))
	mockUrl = mockServer.URL
	defer mockServer.Close()

	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", "https://management.core.windows.net/.default").Return("accessToken", nil)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	// Retrieve the token from the cache
	token, err := creds.GetPassword()
	require.NoError(t, err)
	assert.Equal(t, "newRefreshToken", token, "The retrieved token should match the stored token")
}

func TestChallengeAzureContainerRegistry(t *testing.T) {
	// Set up the mock server
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/", r.URL.Path)
		w.Header().Set("Www-Authenticate", `Bearer realm="https://login.microsoftonline.com/",service="registry.example.com"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	tokenParams, err := creds.challengeAzureContainerRegistry(creds.repoUrl)
	require.NoError(t, err)

	expectedParams := map[string]string{
		"realm":   "https://login.microsoftonline.com/",
		"service": "registry.example.com",
	}
	assert.Equal(t, expectedParams, tokenParams)
}

func TestChallengeAzureContainerRegistryNoChallenge(t *testing.T) {
	// Set up the mock server without Www-Authenticate header
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Replace the real URL with the mock server URL
	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	_, err := creds.challengeAzureContainerRegistry(creds.repoUrl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not issue a challenge")
}

func TestChallengeAzureContainerRegistryNonBearer(t *testing.T) {
	// Set up the mock server with a non-Bearer Www-Authenticate header
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/", r.URL.Path)
		w.Header().Set("Www-Authenticate", `Basic realm="example"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	// Replace the real URL with the mock server URL
	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	_, err := creds.challengeAzureContainerRegistry(creds.repoUrl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not allow 'Bearer' authentication")
}

func TestChallengeAzureContainerRegistryNoService(t *testing.T) {
	// Set up the mock server with a non-Bearer Www-Authenticate header
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/", r.URL.Path)
		w.Header().Set("Www-Authenticate", `Bearer realm="example"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	// Replace the real URL with the mock server URL
	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	_, err := creds.challengeAzureContainerRegistry(creds.repoUrl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service parameter not found in challenge")
}

func TestChallengeAzureContainerRegistryNoRealm(t *testing.T) {
	// Set up the mock server with a non-Bearer Www-Authenticate header
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/", r.URL.Path)
		w.Header().Set("Www-Authenticate", `Bearer service="example"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	// Replace the real URL with the mock server URL
	workloadIdentityMock := new(mocks.TokenProvider)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	_, err := creds.challengeAzureContainerRegistry(creds.repoUrl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "realm parameter not found in challenge")
}

func TestGetAccessTokenAfterChallenge_Success(t *testing.T) {
	// Mock the server to return a successful response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth2/exchange", r.URL.Path)

		response := `{"refresh_token":"newRefreshToken"}`
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(response))
		require.NoError(t, err)
	}))
	defer mockServer.Close()

	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", "https://management.core.windows.net/.default").Return("accessToken", nil)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	tokenParams := map[string]string{
		"realm":   mockServer.URL,
		"service": "registry.example.com",
	}

	refreshToken, err := creds.getAccessTokenAfterChallenge(tokenParams)
	require.NoError(t, err)
	assert.Equal(t, "newRefreshToken", refreshToken)
}

func TestGetAccessTokenAfterChallenge_Failure(t *testing.T) {
	// Mock the server to return an error response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth2/exchange", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"error": "invalid_request"}`))
		require.NoError(t, err)
	}))
	defer mockServer.Close()

	// Create an instance of AzureWorkloadIdentityCreds with the mock credential wrapper
	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", "https://management.core.windows.net/.default").Return("accessToken", nil)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	tokenParams := map[string]string{
		"realm":   mockServer.URL,
		"service": "registry.example.com",
	}

	refreshToken, err := creds.getAccessTokenAfterChallenge(tokenParams)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get refresh token")
	assert.Empty(t, refreshToken)
}

func TestGetAccessTokenAfterChallenge_MalformedResponse(t *testing.T) {
	// Mock the server to return a malformed JSON response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth2/exchange", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"refresh_token":`))
		require.NoError(t, err)
	}))
	defer mockServer.Close()

	// Create an instance of AzureWorkloadIdentityCreds with the mock credential wrapper
	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", "https://management.core.windows.net/.default").Return("accessToken", nil)
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL[8:], "", nil, nil, true, workloadIdentityMock)

	tokenParams := map[string]string{
		"realm":   mockServer.URL,
		"service": "registry.example.com",
	}

	refreshToken, err := creds.getAccessTokenAfterChallenge(tokenParams)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response body")
	assert.Empty(t, refreshToken)
}
