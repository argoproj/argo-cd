package commands

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpccreds "google.golang.org/grpc/credentials"

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

func TestRevokeServerToken_InvalidURL(t *testing.T) {
	// A hostname containing a control character produces an invalid URL,
	// causing http.NewRequestWithContext to fail.
	res, err := revokeServerToken("http", "invalid\x00host", "test-token", false)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "invalid control character in URL")
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

// createTestLocalConfig creates a temporary local config file with a single context
// pointing to the given address and returns the config file path.
func createTestLocalConfig(t *testing.T, addr string) string {
	t.Helper()
	configFile := filepath.Join(t.TempDir(), "config")
	cfg := localconfig.LocalConfig{
		CurrentContext: addr,
		Contexts:       []localconfig.ContextRef{{Name: addr, Server: addr, User: addr}},
		Servers:        []localconfig.Server{{Server: addr}},
		Users:          []localconfig.User{{Name: addr, AuthToken: "test-token"}},
	}
	err := localconfig.WriteLocalConfig(cfg, configFile)
	require.NoError(t, err)
	return configFile
}

// generateSelfSignedCert creates a self-signed TLS certificate for 127.0.0.1.
func generateSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
}

// TestLogout_TLSCheckFails_AutoSetsPlainText verifies that when grpc_util.TestTLS
// returns an error (e.g., nothing listening), the logout command automatically
// switches to plain text and still completes the logout successfully.
func TestLogout_TLSCheckFails_AutoSetsPlainText(t *testing.T) {
	// Listen on a random port, then close it immediately so nothing is listening.
	// This causes grpc_util.TestTLS to fail, triggering the PlainText auto-set path.
	lc := net.ListenConfig{}
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err, "Unable to start server")
	addr := lis.Addr().String()
	lis.Close()

	configFile := createTestLocalConfig(t, addr)

	// Verify token exists before logout
	localCfg, err := localconfig.ReadLocalConfig(configFile)
	require.NoError(t, err)
	assert.Equal(t, "test-token", localCfg.GetToken(addr))

	command := NewLogoutCommand(&argocdclient.ClientOptions{ConfigPath: configFile})
	command.Run(nil, []string{addr})

	// Verify token was removed despite TLS check failure
	localCfg, err = localconfig.ReadLocalConfig(configFile)
	require.NoError(t, err)
	assert.Empty(t, localCfg.GetToken(addr))
}

// TestLogout_InsecureTLS_SetsInsecureFlag verifies that when the server has an
// insecure (self-signed) TLS certificate, the logout command detects it via
// grpc_util.TestTLS (which sets InsecureErr), prompts the user, and sets the
// Insecure flag to true before proceeding with logout.
func TestLogout_InsecureTLS_SetsInsecureFlag(t *testing.T) {
	// Start a gRPC server with TLS credentials using a self-signed certificate.
	// grpc_util.TestTLS will:
	//   1. Connect with InsecureSkipVerify=true → succeeds (TLS=true)
	//   2. Connect with InsecureSkipVerify=false → fails (InsecureErr set)
	cert := generateSelfSignedCert(t)

	lc := net.ListenConfig{}
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err, "Unable to start server")

	serverCreds := grpccreds.NewServerTLSFromCert(&cert)
	grpcServer := grpc.NewServer(grpc.Creds(serverCreds))
	go func() {
		err := grpcServer.Serve(lis)
		require.NoError(t, err, "Unable to start the grpc server")
	}()
	defer grpcServer.Stop()

	addr := lis.Addr().String()

	// Mock os.Stdin to provide "y\n" for cli.AskToProceed (insecure cert warning).
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString("y\n")
	require.NoError(t, err)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	configFile := createTestLocalConfig(t, addr)

	// Verify token exists before logout
	localCfg, err := localconfig.ReadLocalConfig(configFile)
	require.NoError(t, err)
	assert.Equal(t, "test-token", localCfg.GetToken(addr))

	command := NewLogoutCommand(&argocdclient.ClientOptions{ConfigPath: configFile})
	command.Run(nil, []string{addr})

	// Verify token was removed after accepting the insecure cert warning
	localCfg, err = localconfig.ReadLocalConfig(configFile)
	require.NoError(t, err)
	assert.Empty(t, localCfg.GetToken(addr))
}

// TestLogout_ConfigFileNotFound verifies that when the local config file does not
// exist, the logout command reports "Nothing to logout from" via log.Fatalf.
func TestLogout_ConfigFileNotFound(t *testing.T) {
	// Point to a config path that does not exist.
	configFile := filepath.Join(t.TempDir(), "nonexistent", "config")

	// Override logrus ExitFunc so log.Fatalf panics instead of calling os.Exit,
	// allowing us to verify the fatal message in the test.
	origExitFunc := logrus.StandardLogger().ExitFunc
	logrus.StandardLogger().ExitFunc = func(code int) {
		panic(fmt.Sprintf("os.Exit(%d)", code))
	}
	defer func() { logrus.StandardLogger().ExitFunc = origExitFunc }()

	// Capture log output to verify the error message.
	var buf bytes.Buffer
	origOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(origOutput)

	assert.Panics(t, func() {
		command := NewLogoutCommand(&argocdclient.ClientOptions{ConfigPath: configFile})
		command.Run(nil, []string{"some-context"})
	})

	assert.Contains(t, buf.String(), "Nothing to logout from")
}
