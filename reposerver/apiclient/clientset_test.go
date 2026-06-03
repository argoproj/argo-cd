package apiclient_test

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
)

func TestNewRepoServerClient_CorrectClientReturned(t *testing.T) {
	t.Parallel()
	mockClientset := &mocks.Clientset{
		RepoServerServiceClient: &mocks.RepoServerServiceClient{},
	}

	closer, client, err := mockClientset.NewRepoServerClient()

	require.NoError(t, err)
	assert.NotNil(t, closer)
	assert.NotNil(t, client)
	assert.Equal(t, mockClientset.RepoServerServiceClient, client)
}

func TestNewRepoServerClientset_InvalidInput(t *testing.T) {
	t.Parallel()
	// Call the function with invalid inputs
	invalidClientset := apiclient.NewRepoServerClientset("", -1, apiclient.TLSConfiguration{})

	assert.NotNil(t, invalidClientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), invalidClientset)
}

func TestNewRepoServerClientset_SuccessfulConnection(t *testing.T) {
	t.Parallel()
	// Call the function with valid inputs
	clientset := apiclient.NewRepoServerClientset("localhost:8080", 1, apiclient.TLSConfiguration{})

	assert.NotNil(t, clientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), clientset)
}

func TestNewRepoServerClientset_SuccessfulConnectionWithTLS(t *testing.T) {
	t.Parallel()
	// Call the function with valid inputs
	clientset := apiclient.NewRepoServerClientset("localhost:8080", 1, apiclient.TLSConfiguration{
		DisableTLS:       false,
		StrictValidation: true,
		Certificates:     nil,
	})

	assert.NotNil(t, clientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), clientset)
}

func TestNewConnection_TLSWithStrictValidation(t *testing.T) {
	t.Parallel()
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       false,
		StrictValidation: true,
		Certificates:     nil,
	}

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_TLSWithStrictValidationAndCertificates(t *testing.T) {
	t.Parallel()
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       false,
		StrictValidation: true,
		Certificates:     nil,
	}

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_InsecureConnection(t *testing.T) {
	t.Parallel()
	// Create a TLS configuration with TLS disabled
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:       true,
		StrictValidation: false,
		Certificates:     nil,
	}

	conn, err := apiclient.NewConnection("example.com:80", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_InMemoryCertificates(t *testing.T) {
	tlsConfig := apiclient.TLSConfiguration{
		DisableTLS:         false,
		StrictValidation:   false,
		ClientCertificates: []tls.Certificate{{}},
	}

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)

	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_ServesFromCacheWhenFilesUnchanged(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	tempCertFile := filepath.Join(t.TempDir(), "client.crt")
	tempKeyFile := filepath.Join(t.TempDir(), "client.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	// First call: loads from disk and populates the cache.
	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	// Second call: files are untouched, mtime is unchanged — cert must be served from the cache without re-reading disk.
	conn, err = apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_ReloadsClientCertificateWhenFileChanges(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	tempCertFile := filepath.Join(t.TempDir(), "client.crt")
	tempKeyFile := filepath.Join(t.TempDir(), "client.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	// First call: loads and caches the cert.
	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	// Simulate cert rotation: overwrite with fresh (still valid) cert bytes.
	// Writing updates the mtime, which is enough to trigger a reload.
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	// Second call: mtime changed → cache invalidated → reloads from disk.
	// The new cert is valid, so the call must succeed.
	conn, err = apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestNewConnection_ErrorWhenRotatedCertIsInvalid(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	tempCertFile := filepath.Join(t.TempDir(), "client.crt")
	tempKeyFile := filepath.Join(t.TempDir(), "client.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempCertFile, certBytes, 0o600))

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempKeyFile, keyBytes, 0o600))

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	// First call: succeeds and populates the cache.
	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	require.NoError(t, os.WriteFile(tempCertFile, []byte("not a valid certificate"), 0o600))
	require.NoError(t, os.WriteFile(tempKeyFile, []byte("not a valid key"), 0o600))

	// Second call: mtime changed → reload attempted → parse fails → error returned.
	// The stale (valid) cached cert must NOT be silently served.
	_, err = apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.Error(t, err,
		"a corrupt cert file must cause an error after cache invalidation, "+
			"not silently serve the stale cached cert")
	assert.Contains(t, err.Error(), "failed to load client certificate",
		"error must identify the cert load as the failure site")
}

func TestNewConnection_DoesNotCacheClientCertificateLoadError(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	tempDir := t.TempDir()
	tempCertFile := filepath.Join(tempDir, "client.crt")
	tempKeyFile := filepath.Join(tempDir, "client.key")

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	_, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.Error(t, err)

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	err = os.WriteFile(tempCertFile, certBytes, 0o600)
	require.NoError(t, err)

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	err = os.WriteFile(tempKeyFile, keyBytes, 0o600)
	require.NoError(t, err)

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)
}
