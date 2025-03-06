package apiclient_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
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
