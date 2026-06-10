package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPTemplateAuthenticator_OctoSTS_Style(t *testing.T) {
	// Simulates octo-sts: GET with Bearer auth, returns access_token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		assert.Equal(t, http.MethodGet, r.Method)

		// Verify Bearer auth
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer k8s-sa-token", auth)

		// Verify path and query params
		assert.Equal(t, "/sts/exchange", r.URL.Path)
		assert.Equal(t, "myorg/myrepo", r.URL.Query().Get("scope"))
		assert.Equal(t, "argocd", r.URL.Query().Get("identity"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "github-installation-token",
		})
	}))
	defer server.Close()

	// Extract host from test server
	host := server.URL[7:] // strip "http://"

	a := NewHTTPTemplateAuthenticator()
	token := &Token{
		Type:  TokenTypeBearer,
		Token: "k8s-sa-token",
	}

	config := &Config{
		Method:       "GET",
		PathTemplate: "/sts/exchange?scope={{ .repo }}&identity={{ .policy }}",
		AuthType:     "bearer",
		Params: map[string]string{
			"policy": "argocd",
		},
		Username: "x-access-token",
		Insecure: true,
	}

	creds, err := a.Authenticate(context.Background(), token, "oci://"+host+"/myorg/myrepo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "x-access-token", creds.Username)
	assert.Equal(t, "github-installation-token", creds.Password)
}

func TestHTTPTemplateAuthenticator_ACR_Style(t *testing.T) {
	// Simulates ACR: POST form with token in body, returns refresh_token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify no Authorization header (token is in body)
		assert.Empty(t, r.Header.Get("Authorization"))

		// Verify content type
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		// Parse form
		err := r.ParseForm()
		require.NoError(t, err)

		assert.Equal(t, "access_token", r.Form.Get("grant_type"))
		assert.Contains(t, r.Form.Get("service"), "127.0.0.1") // test server host
		assert.Equal(t, "azure-access-token", r.Form.Get("access_token"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"refresh_token": "acr-refresh-token",
		})
	}))
	defer server.Close()

	host := server.URL[7:] // strip "http://"

	a := NewHTTPTemplateAuthenticator()
	token := &Token{
		Type:  TokenTypeBearer,
		Token: "azure-access-token",
	}

	config := &Config{
		Method:             "POST",
		PathTemplate:       "/oauth2/exchange",
		BodyTemplate:       "grant_type=access_token&service={{ .registry }}&access_token={{ .token }}",
		AuthType:           "none",
		ResponseTokenField: "refresh_token",
		Username:           "00000000-0000-0000-0000-000000000000",
		Insecure:           true,
	}

	creds, err := a.Authenticate(context.Background(), token, "oci://"+host+"/myrepo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", creds.Username)
	assert.Equal(t, "acr-refresh-token", creds.Password)
}

func TestHTTPTemplateAuthenticator_JFrog_Style(t *testing.T) {
	// Simulates JFrog: POST JSON with token in body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify content type
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse JSON body
		var body map[string]string
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)

		assert.Equal(t, "urn:ietf:params:oauth:grant-type:token-exchange", body["grant_type"])
		assert.Equal(t, "urn:ietf:params:oauth:token-type:id_token", body["subject_token_type"])
		assert.Equal(t, "k8s-jwt-token", body["subject_token"])
		assert.Equal(t, "my-oidc-provider", body["provider_name"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "jfrog-access-token",
		})
	}))
	defer server.Close()

	host := server.URL[7:] // strip "http://"

	a := NewHTTPTemplateAuthenticator()
	token := &Token{
		Type:  TokenTypeBearer,
		Token: "k8s-jwt-token",
	}

	config := &Config{
		Method:       "POST",
		PathTemplate: "/access/api/v1/oidc/token",
		BodyTemplate: `{"grant_type":"urn:ietf:params:oauth:grant-type:token-exchange","subject_token_type":"urn:ietf:params:oauth:token-type:id_token","subject_token":"{{ .token }}","provider_name":"{{ .provider }}"}`,
		AuthType:     "none",
		Params: map[string]string{
			"provider": "my-oidc-provider",
		},
		Insecure: true,
	}

	creds, err := a.Authenticate(context.Background(), token, "oci://"+host+"/myrepo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "$oauthtoken", creds.Username) // default
	assert.Equal(t, "jfrog-access-token", creds.Password)
}

func TestHTTPTemplateAuthenticator_Quay_Style(t *testing.T) {
	// Simulates Quay robot federation: GET with Basic auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		assert.Equal(t, http.MethodGet, r.Method)

		// Verify Basic auth
		username, password, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "myorg+robot", username)
		assert.Equal(t, "k8s-jwt-token", password)

		// Verify query params
		assert.Contains(t, r.URL.Query().Get("service"), "127.0.0.1")
		assert.Equal(t, "repository:myorg/myrepo:pull", r.URL.Query().Get("scope"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": "quay-registry-token",
		})
	}))
	defer server.Close()

	host := server.URL[7:] // strip "http://"

	a := NewHTTPTemplateAuthenticator()
	token := &Token{
		Type:  TokenTypeBearer,
		Token: "k8s-jwt-token",
	}

	config := &Config{
		Method:             "GET",
		PathTemplate:       "/v2/auth?service={{ .registry }}&scope={{ .scope }}",
		AuthType:           "basic",
		Username:           "myorg+robot",
		ResponseTokenField: "token",
		Params: map[string]string{
			"scope": "repository:myorg/myrepo:pull",
		},
		Insecure: true,
	}

	creds, err := a.Authenticate(context.Background(), token, "oci://"+host+"/myorg/myrepo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "myorg+robot", creds.Username)
	assert.Equal(t, "quay-registry-token", creds.Password)
}

func TestHTTPTemplateAuthenticator_AuthHostOverride(t *testing.T) {
	// Simulates octo-sts: auth endpoint is on different host than registry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer k8s-sa-token", r.Header.Get("Authorization"))

		// Verify the repo path from ghcr.io is still available in query
		assert.Equal(t, "myorg/myrepo", r.URL.Query().Get("scope"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "github-installation-token",
		})
	}))
	defer server.Close()

	// Auth host is the test server, but registry is ghcr.io
	authHost := server.URL[7:] // strip "http://"

	a := NewHTTPTemplateAuthenticator()
	token := &Token{
		Type:  TokenTypeBearer,
		Token: "k8s-sa-token",
	}

	config := &Config{
		AuthHost:     authHost, // Override: send request here instead of ghcr.io
		Method:       "GET",
		PathTemplate: "/sts/exchange?scope={{ .repo }}&identity={{ .policy }}",
		AuthType:     "bearer",
		Params: map[string]string{
			"policy": "argocd",
		},
		Username: "x-access-token",
		Insecure: true,
	}

	// Repo URL points to ghcr.io, but auth request goes to authHost (test server)
	creds, err := a.Authenticate(context.Background(), token, "oci://ghcr.io/myorg/myrepo", config)

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "x-access-token", creds.Username)
	assert.Equal(t, "github-installation-token", creds.Password)
}

func TestHTTPTemplateAuthenticator_DefaultMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default should be GET
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "test-token",
		})
	}))
	defer server.Close()

	host := server.URL[7:]

	a := NewHTTPTemplateAuthenticator()
	token := &Token{Type: TokenTypeBearer, Token: "jwt"}

	config := &Config{
		PathTemplate: "/auth",
		// Method not set - should default to GET
		Insecure: true,
	}

	creds, err := a.Authenticate(context.Background(), token, "oci://"+host+"/repo", config)

	require.NoError(t, err)
	assert.Equal(t, "test-token", creds.Password)
}

func TestHTTPTemplateAuthenticator_MissingPathTemplate(t *testing.T) {
	a := NewHTTPTemplateAuthenticator()
	token := &Token{Type: TokenTypeBearer, Token: "jwt"}

	config := &Config{
		// PathTemplate not set
	}

	_, err := a.Authenticate(context.Background(), token, "oci://registry.example.com/repo", config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pathTemplate is required")
}

func TestHTTPTemplateAuthenticator_WrongTokenType(t *testing.T) {
	a := NewHTTPTemplateAuthenticator()
	token := &Token{
		Type:           TokenTypeAWS,
		AWSCredentials: &AWSCredentials{AccessKeyID: "AKIA..."},
	}

	config := &Config{
		PathTemplate: "/auth",
	}

	_, err := a.Authenticate(context.Background(), token, "oci://registry.example.com/repo", config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a bearer token")
}

func TestHTTPTemplateAuthenticator_BasicAuthWithoutUsername(t *testing.T) {
	a := NewHTTPTemplateAuthenticator()
	token := &Token{Type: TokenTypeBearer, Token: "jwt"}

	config := &Config{
		PathTemplate: "/auth",
		AuthType:     "basic",
		// Username not set
	}

	_, err := a.Authenticate(context.Background(), token, "oci://registry.example.com/repo", config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "username is required for basic auth")
}

func TestHTTPTemplateAuthenticator_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid token"}`))
	}))
	defer server.Close()

	host := server.URL[7:]

	a := NewHTTPTemplateAuthenticator()
	token := &Token{Type: TokenTypeBearer, Token: "bad-token"}

	config := &Config{
		PathTemplate: "/auth",
		Insecure:     true,
	}

	_, err := a.Authenticate(context.Background(), token, "oci://"+host+"/repo", config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestHTTPTemplateAuthenticator_NoTokenInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"error": "something went wrong",
		})
	}))
	defer server.Close()

	host := server.URL[7:]

	a := NewHTTPTemplateAuthenticator()
	token := &Token{Type: TokenTypeBearer, Token: "jwt"}

	config := &Config{
		PathTemplate: "/auth",
		Insecure:     true,
	}

	_, err := a.Authenticate(context.Background(), token, "oci://"+host+"/repo", config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no token field found")
}

func TestSubstituteTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		vars      map[string]string
		expected  string
		expectErr bool
	}{
		{
			name:     "simple substitution",
			template: "/auth?scope={{ .scope }}",
			vars:     map[string]string{"scope": "repo:pull"},
			expected: "/auth?scope=repo:pull",
		},
		{
			name:     "multiple substitutions",
			template: "/auth?service={{ .registry }}&scope={{ .scope }}",
			vars:     map[string]string{"registry": "quay.io", "scope": "repo:pull"},
			expected: "/auth?service=quay.io&scope=repo:pull",
		},
		{
			name:     "JSON body",
			template: `{"token":"{{ .token }}","provider":"{{ .provider }}"}`,
			vars:     map[string]string{"token": "abc123", "provider": "k8s"},
			expected: `{"token":"abc123","provider":"k8s"}`,
		},
		{
			name:     "sprig function urlquery",
			template: "/auth?scope={{ .scope | urlquery }}",
			vars:     map[string]string{"scope": "repo:pull,push"},
			expected: "/auth?scope=repo%3Apull%2Cpush",
		},
		{
			name:     "sprig function upper",
			template: "/auth?method={{ .method | upper }}",
			vars:     map[string]string{"method": "get"},
			expected: "/auth?method=GET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := substituteTemplate(tt.template, tt.vars)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name             string
		repoURL          string
		expectedRegistry string
		expectedPath     string
		expectErr        bool
	}{
		{
			name:             "OCI URL",
			repoURL:          "oci://quay.io/myorg/myrepo",
			expectedRegistry: "quay.io",
			expectedPath:     "myorg/myrepo",
		},
		{
			name:             "HTTPS URL",
			repoURL:          "https://registry.example.com/namespace/repo",
			expectedRegistry: "registry.example.com",
			expectedPath:     "namespace/repo",
		},
		{
			name:             "deeply nested",
			repoURL:          "oci://registry.example.com/org/team/project/repo",
			expectedRegistry: "registry.example.com",
			expectedPath:     "org/team/project/repo",
		},
		{
			name:             "no path",
			repoURL:          "oci://registry.example.com",
			expectedRegistry: "registry.example.com",
			expectedPath:     "",
		},
		{
			name:             "with port",
			repoURL:          "oci://registry.example.com:5000/myrepo",
			expectedRegistry: "registry.example.com:5000",
			expectedPath:     "myrepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, repoPath, err := parseRepoURL(tt.repoURL)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedRegistry, registry)
				assert.Equal(t, tt.expectedPath, repoPath)
			}
		})
	}
}

func TestExtractTokenFromResponse(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		field     string
		expected  string
		expectErr bool
	}{
		{
			name:     "access_token field",
			body:     `{"access_token": "abc123"}`,
			field:    "",
			expected: "abc123",
		},
		{
			name:     "token field",
			body:     `{"token": "abc123"}`,
			field:    "",
			expected: "abc123",
		},
		{
			name:     "refresh_token field",
			body:     `{"refresh_token": "abc123"}`,
			field:    "",
			expected: "abc123",
		},
		{
			name:     "specific field",
			body:     `{"custom_token": "abc123"}`,
			field:    "custom_token",
			expected: "abc123",
		},
		{
			name:      "missing specific field",
			body:      `{"other": "value"}`,
			field:     "custom_token",
			expectErr: true,
		},
		{
			name:      "no known fields",
			body:      `{"error": "something"}`,
			field:     "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractTokenFromResponse([]byte(tt.body), tt.field)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestHTTPTemplateAuthenticator_ImplementsInterface(t *testing.T) {
	var _ Authenticator = (*HTTPTemplateAuthenticator)(nil)
}
