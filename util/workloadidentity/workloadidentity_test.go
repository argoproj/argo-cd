package workloadidentity

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockTokenCredential struct {
	mockedToken string
	mockedError error
}

func (c MockTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: c.mockedToken}, c.mockedError
}

func TestNewWorkloadIdentityTokenProvider_Success(t *testing.T) {
	// Replace the initialization with the mock
	initError = nil
	provider := WorkloadIdentityTokenProvider{tokenCredential: MockTokenCredential{}}

	// Test the NewWorkloadIdentityTokenProvider function
	_, err := provider.GetToken("https://management.core.windows.net/.default")
	require.NoError(t, err, "Expected no error from GetToken")
}

func TestGetToken_Success(t *testing.T) {
	initError = nil
	provider := WorkloadIdentityTokenProvider{tokenCredential: MockTokenCredential{mockedToken: "mocked_token"}}
	scope := "https://management.core.windows.net/.default"

	token, err := provider.GetToken(scope)
	require.NoError(t, err, "Expected no error from GetToken")
	assert.Equal(t, "mocked_token", token, "Expected token to match")
}

func TestGetToken_Failure(t *testing.T) {
	initError = nil
	provider := WorkloadIdentityTokenProvider{tokenCredential: MockTokenCredential{mockedToken: "mocked_token", mockedError: errors.New("Expected error from GetToken")}}
	scope := "https://management.core.windows.net/.default"

	token, err := provider.GetToken(scope)
	require.Error(t, err, "Expected error from GetToken")
	assert.Empty(t, token, "Expected token to be empty on error")
}

func TestGetToken_InitError(t *testing.T) {
	initError = errors.New("initialization error")
	provider := WorkloadIdentityTokenProvider{tokenCredential: MockTokenCredential{mockedToken: "mocked_token", mockedError: errors.New("Expected error from GetToken")}}

	token, err := provider.GetToken("https://management.core.windows.net/.default")
	require.Error(t, err, "Expected error from GetToken due to initialization error")
	assert.Empty(t, token, "Expected token to be empty on initialization error")
}
