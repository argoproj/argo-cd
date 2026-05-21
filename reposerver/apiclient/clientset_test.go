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
	// Call the function with invalid inputs
	invalidClientset := apiclient.NewRepoServerClientset("", -1, apiclient.TLSConfiguration{})

	assert.NotNil(t, invalidClientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), invalidClientset)
}

func TestNewRepoServerClientset_SuccessfulConnection(t *testing.T) {
	// Call the function with valid inputs
	clientset := apiclient.NewRepoServerClientset("localhost:8080", 1, apiclient.TLSConfiguration{})

	assert.NotNil(t, clientset)
	assert.Implements(t, (*apiclient.Clientset)(nil), clientset)
}

func TestNewRepoServerClientset_SuccessfulConnectionWithTLS(t *testing.T) {
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

func TestNewConnection_CachesClientCertificateFromFiles(t *testing.T) {
	t.Cleanup(apiclient.ResetClientCertCache)
	apiclient.ResetClientCertCache()

	certFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.crt")
	keyFile := filepath.Join("..", "..", "util", "tls", "testdata", "valid_tls.key")

	tempCertFile := filepath.Join(t.TempDir(), "client.crt")
	tempKeyFile := filepath.Join(t.TempDir(), "client.key")

	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	err = os.WriteFile(tempCertFile, certBytes, 0o600)
	require.NoError(t, err)

	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	err = os.WriteFile(tempKeyFile, keyBytes, 0o600)
	require.NoError(t, err)

	tlsConfig := apiclient.TLSConfiguration{
		StrictValidation:  false,
		ClientCertFile:    tempCertFile,
		ClientCertKeyFile: tempKeyFile,
	}

	conn, err := apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	err = os.WriteFile(tempCertFile, []byte("invalid cert"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(tempKeyFile, []byte("invalid key"), 0o600)
	require.NoError(t, err)

	conn, err = apiclient.NewConnection("example.com:443", 10, &tlsConfig)
	require.NoError(t, err)
	assert.NotNil(t, conn)
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
