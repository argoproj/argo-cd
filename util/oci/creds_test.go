package oci

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoutils "github.com/argoproj/argo-cd/v3/util"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/mocks"
)

func TestOCICredsGetUsernameAndPassword(t *testing.T) {
	creds := OCICreds{
		Username: "testuser",
		Password: "testpass",
	}
	assert.Equal(t, "testuser", creds.GetUsername())
	password, err := creds.GetPassword()
	require.NoError(t, err)
	assert.Equal(t, "testpass", password)
}

func TestOCICredsCA(t *testing.T) {
	creds := OCICreds{
		CAPath:   "/path/to/ca",
		CertData: []byte("cert-data"),
		KeyData:  []byte("key-data"),
	}
	assert.Equal(t, "/path/to/ca", creds.GetCAPath())
	assert.Equal(t, []byte("cert-data"), creds.GetCertData())
	assert.Equal(t, []byte("key-data"), creds.GetKeyData())
}

func TestOCICredsGetInsecure(t *testing.T) {
	creds := OCICreds{
		InsecureSkipVerify: true,
		InsecureHTTPOnly:   true,
	}
	assert.True(t, creds.GetInsecureSkipVerify())
	assert.True(t, creds.GetInsecureHTTPOnly())
}

func TestWorkLoadIdentityUserNameShouldBeEmptyGuid(t *testing.T) {
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io", "", nil, nil, false, false, workloadIdentityMock)
	username := creds.GetUsername()

	assert.Equal(t, workloadidentity.EmptyGuid, username, "The username for azure workload identity is not empty Guid")
}

func TestAzureWorkloadIdentityCredsGetCAPath(t *testing.T) {
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io", "/path/to/ca", nil, nil, false, false, workloadIdentityMock)
	assert.Equal(t, "/path/to/ca", creds.GetCAPath())
}

func TestAzureWorkloadIdentityCredsGetCertData(t *testing.T) {
	certData := []byte("cert-data")
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io", "", certData, nil, false, false, workloadIdentityMock)
	assert.Equal(t, certData, creds.GetCertData())
}

func TestAzureWorkloadIdentityCredsGetKeyData(t *testing.T) {
	keyData := []byte("key-data")
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io", "", nil, keyData, false, false, workloadIdentityMock)
	assert.Equal(t, keyData, creds.GetKeyData())
}

func TestAzureWorkloadIdentityCredsGetInsecureSkipVerify(t *testing.T) {
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io", "", nil, nil, true, false, workloadIdentityMock)
	assert.True(t, creds.GetInsecureSkipVerify())
}

func TestAzureWorkloadIdentityCredsGetInsecureHTTPOnly(t *testing.T) {
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("contoso.azurecr.io", "", nil, nil, false, true, workloadIdentityMock)
	assert.True(t, creds.GetInsecureHTTPOnly())
}

func TestGetAccessTokenShouldReturnTokenFromCacheIfPresent(t *testing.T) {
	resetAzureTokenCache()
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("https://contoso.azurecr.io", "", nil, nil, false, false, workloadIdentityMock)

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
	resetAzureTokenCache()
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("https://contoso.azurecr.io", "", nil, nil, false, false, workloadIdentityMock)

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
	resetAzureTokenCache()
	mockServerURL := ""
	mockedServerURL := func() string {
		return mockServerURL
	}

	// Mock the server to return a successful response
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm=%q,service=%q`, mockedServerURL(), mockedServerURL()[8:]))
			w.WriteHeader(http.StatusUnauthorized)

		case "/oauth2/exchange":
			response := `{"refresh_token":"newRefreshToken"}`
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(response))
			require.NoError(t, err)
		}
	}))
	mockServerURL = mockServer.URL
	defer mockServer.Close()

	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken("https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil).Maybe()
	// Use the full URL for OCI repo
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

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

	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	// Parse the host from the URL
	parsed, _ := url.Parse(creds.repoURL)
	tokenParams, err := creds.challengeAzureContainerRegistry(t.Context(), parsed.Host)
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
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	// Parse the host from the URL
	parsed, _ := url.Parse(creds.repoURL)
	_, err := creds.challengeAzureContainerRegistry(t.Context(), parsed.Host)
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
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	// Parse the host from the URL
	parsed, _ := url.Parse(creds.repoURL)
	_, err := creds.challengeAzureContainerRegistry(t.Context(), parsed.Host)
	assert.ErrorContains(t, err, "does not allow 'Bearer' authentication")
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
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	// Parse the host from the URL
	parsed, _ := url.Parse(creds.repoURL)
	_, err := creds.challengeAzureContainerRegistry(t.Context(), parsed.Host)
	assert.ErrorContains(t, err, "service parameter not found in challenge")
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
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	// Parse the host from the URL
	parsed, _ := url.Parse(creds.repoURL)
	_, err := creds.challengeAzureContainerRegistry(t.Context(), parsed.Host)
	assert.ErrorContains(t, err, "realm parameter not found in challenge")
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

	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken("https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil).Maybe()
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	tokenParams := map[string]string{
		"realm":   mockServer.URL,
		"service": "registry.example.com",
	}

	refreshToken, err := creds.getAccessTokenAfterChallenge(t.Context(), tokenParams)
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
	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken("https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil).Maybe()
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	tokenParams := map[string]string{
		"realm":   mockServer.URL,
		"service": "registry.example.com",
	}

	refreshToken, err := creds.getAccessTokenAfterChallenge(t.Context(), tokenParams)
	require.ErrorContains(t, err, "failed to get refresh token")
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
	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken("https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil).Maybe()
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	tokenParams := map[string]string{
		"realm":   mockServer.URL,
		"service": "registry.example.com",
	}

	refreshToken, err := creds.getAccessTokenAfterChallenge(t.Context(), tokenParams)
	require.ErrorContains(t, err, "failed to unmarshal response body")
	assert.Empty(t, refreshToken)
}

// Helper to generate a mock JWT token with a given expiry time
func generateMockJWT(expiry time.Time) (string, error) {
	claims := jwt.MapClaims{
		"exp": expiry.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Use a dummy secret for signing
	return token.SignedString([]byte("dummy-secret"))
}

func TestGetJWTExpiry_Success(t *testing.T) {
	expiry := time.Now().Add(1 * time.Hour)
	token, err := generateMockJWT(expiry)
	require.NoError(t, err)

	extractedExpiry, err := getJWTExpiry(token)
	require.NoError(t, err)
	assert.Equal(t, expiry.Unix(), extractedExpiry.Unix())
}

func TestGetJWTExpiry_InvalidToken(t *testing.T) {
	_, err := getJWTExpiry("invalid.token.here")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JWT")
}

func TestGetJWTExpiry_NoExpClaim(t *testing.T) {
	// Create a token without exp claim
	claims := jwt.MapClaims{
		"sub": "1234567890",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("dummy-secret"))
	require.NoError(t, err)

	_, err = getJWTExpiry(tokenString)
	require.Error(t, err)
}

func TestGetAccessToken_FetchNewTokenIfExistingIsExpired(t *testing.T) {
	resetAzureTokenCache()
	accessToken1, _ := generateMockJWT(time.Now().Add(1 * time.Minute))
	accessToken2, _ := generateMockJWT(time.Now().Add(1 * time.Minute))

	mockServerURL := ""
	mockedServerURL := func() string {
		return mockServerURL
	}

	callCount := 0
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			assert.Equal(t, "/v2/", r.URL.Path)
			w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm=%q,service=%q`, mockedServerURL(), mockedServerURL()[8:]))
			w.WriteHeader(http.StatusUnauthorized)
		case "/oauth2/exchange":
			assert.Equal(t, "/oauth2/exchange", r.URL.Path)
			var response string
			switch callCount {
			case 0:
				response = fmt.Sprintf(`{"refresh_token": %q}`, accessToken1)
			case 1:
				response = fmt.Sprintf(`{"refresh_token": %q}`, accessToken2)
			default:
				response = `{"refresh_token": "defaultToken"}`
			}
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(response))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()
	mockServerURL = mockServer.URL

	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken("https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil).Maybe()
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	refreshToken, err := creds.GetAccessToken()
	require.NoError(t, err)
	assert.Equal(t, accessToken1, refreshToken)

	time.Sleep(5 * time.Second) // Wait for the token to expire

	refreshToken, err = creds.GetAccessToken()
	require.NoError(t, err)
	assert.Equal(t, accessToken2, refreshToken)
}

func TestGetAccessToken_ReuseTokenIfExistingIsNotExpired(t *testing.T) {
	resetAzureTokenCache()
	accessToken1, _ := generateMockJWT(time.Now().Add(6 * time.Minute))
	accessToken2, _ := generateMockJWT(time.Now().Add(1 * time.Minute))

	mockServerURL := ""
	mockedServerURL := func() string {
		return mockServerURL
	}

	callCount := 0
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			assert.Equal(t, "/v2/", r.URL.Path)
			w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm=%q,service=%q`, mockedServerURL(), mockedServerURL()[8:]))
			w.WriteHeader(http.StatusUnauthorized)
		case "/oauth2/exchange":
			assert.Equal(t, "/oauth2/exchange", r.URL.Path)
			var response string
			switch callCount {
			case 0:
				response = fmt.Sprintf(`{"refresh_token": %q}`, accessToken1)
			case 1:
				response = fmt.Sprintf(`{"refresh_token": %q}`, accessToken2)
			default:
				response = `{"refresh_token": "defaultToken"}`
			}
			callCount++
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(response))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()
	mockServerURL = mockServer.URL

	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken("https://management.core.windows.net/.default").Return(&workloadidentity.Token{AccessToken: "accessToken"}, nil).Maybe()
	creds := NewAzureWorkloadIdentityCreds(mockServer.URL, "", nil, nil, true, false, workloadIdentityMock)

	refreshToken, err := creds.GetAccessToken()
	require.NoError(t, err)
	assert.Equal(t, accessToken1, refreshToken)

	time.Sleep(5 * time.Second) // Wait for the token to expire

	refreshToken, err = creds.GetAccessToken()
	require.NoError(t, err)
	assert.Equal(t, accessToken1, refreshToken)
}

func TestGetAccessToken_InvalidRepoURL(t *testing.T) {
	workloadIdentityMock := &mocks.TokenProvider{}
	creds := NewAzureWorkloadIdentityCreds("://invalid-url", "", nil, nil, false, false, workloadIdentityMock)

	_, err := creds.GetAccessToken()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse oci repo url")
}

func resetAzureTokenCache() {
	azureTokenCache = gocache.New(gocache.NoExpiration, 0)
}
