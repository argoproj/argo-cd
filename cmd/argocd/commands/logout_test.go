package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

func TestLogout(t *testing.T) {
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testConfig), os.ModePerm)
	require.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0o600)
	require.NoError(t, err)

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})

	command := NewLogoutCommand(&argocdclient.ClientOptions{ConfigPath: testConfigFilePath})
	command.Run(nil, []string{"localhost:8080"})

	localConfig, err = localconfig.ReadLocalConfig(testConfigFilePath)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", localConfig.CurrentContext)
	assert.NotContains(t, localConfig.Users, localconfig.User{AuthToken: "vErrYS3c3tReFRe$hToken", Name: "localhost:8080"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd1.example.com:443", Server: "argocd1.example.com:443", User: "argocd1.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "argocd2.example.com:443", Server: "argocd2.example.com:443", User: "argocd2.example.com:443"})
	assert.Contains(t, localConfig.Contexts, localconfig.ContextRef{Name: "localhost:8080", Server: "localhost:8080", User: "localhost:8080"})
}

func TestRevokeServerToken_EmptyToken(t *testing.T) {
	res, err := revokeServerToken("http", "localhost:8080", "", false)
	require.EqualError(t, err, "error getting token from local context file")
	assert.Nil(t, res)
}

func TestRevokeServerToken_SuccessfulRequest(t *testing.T) {
	var receivedCookie string
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		cookie, err := r.Cookie(common.AuthCookieName)
		if err == nil {
			receivedCookie = cookie.Value
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Strip the "http://" prefix to get the hostName
	hostName := server.Listener.Addr().String()

	res, err := revokeServerToken("http", hostName, "test-token", false)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, http.MethodPost, receivedMethod)
	assert.Equal(t, "test-token", receivedCookie)
}

func TestRevokeServerToken_ServerReturnsBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	hostName := server.Listener.Addr().String()

	res, err := revokeServerToken("http", hostName, "test-token", false)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestRevokeServerToken_InsecureTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hostName := server.Listener.Addr().String()

	// Without insecure, the self-signed cert should cause a failure
	res, err := revokeServerToken("https", hostName, "test-token", false)
	require.Error(t, err)
	assert.Nil(t, res)

	// With insecure=true, should succeed despite self-signed cert
	res, err = revokeServerToken("https", hostName, "test-token", true)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}
