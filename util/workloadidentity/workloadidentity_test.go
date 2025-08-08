package workloadidentity

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azcloud "github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
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

func TestNewWorkloadIdentityTokenProviderWithAzureCloud_Success(t *testing.T) {
	clouds := map[string]azcloud.Configuration{
		"AzurePublic":     azcloud.AzurePublic,
		"AzureChina":      azcloud.AzureChina,
		"AzureGovernment": azcloud.AzureGovernment,
	}

	for cloudName, cloud := range clouds {
		t.Run(cloudName, func(t *testing.T) {
			// Mock the newDefaultAzureCredential function to capture the cloud configuration
			var actualCloud azcloud.Configuration
			newDefaultAzureCredential = func(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error) {
				actualCloud = options.ClientOptions.Cloud
				return &azidentity.DefaultAzureCredential{}, nil
			}

			// Restore the original function after the test
			defer func() { newDefaultAzureCredential = azidentity.NewDefaultAzureCredential }()

			NewWorkloadIdentityTokenProvider(cloudName)

			assert.Equal(t, cloud.ActiveDirectoryAuthorityHost, actualCloud.ActiveDirectoryAuthorityHost, fmt.Sprintf("Expected cloud to match %s", cloudName))
		})
	}
}

func TestNewWorkloadIdentityTokenProviderWithInvalidAzureCloud_LogWarning(t *testing.T) {
	// Replace the logger with a test hook to capture log output
	oldHooks := logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})
	hook := logtest.NewGlobal()

	// Mock the newDefaultAzureCredential function
	newDefaultAzureCredential = func(options *azidentity.DefaultAzureCredentialOptions) (*azidentity.DefaultAzureCredential, error) {
		return &azidentity.DefaultAzureCredential{}, nil
	}

	// Restore after test
	defer func() { newDefaultAzureCredential = azidentity.NewDefaultAzureCredential }()
	defer logrus.StandardLogger().ReplaceHooks(oldHooks)

	NewWorkloadIdentityTokenProvider("InvalidCloud")

	lastEntry := hook.LastEntry()
	assert.NotNil(t, lastEntry, t.Name())
	assert.Equal(t, logrus.WarnLevel, lastEntry.Level, t.Name())
	hook.Reset()
}

func TestGetToken_Success(t *testing.T) {
	initError = nil
	provider := WorkloadIdentityTokenProvider{tokenCredential: MockTokenCredential{mockedToken: "mocked_token"}}
	scope := "https://management.core.windows.net/.default"

	token, err := provider.GetToken(scope)
	require.NoError(t, err, "Expected no error from GetToken")
	assert.Equal(t, "mocked_token", token.AccessToken, "Expected token to match")
}

func TestGetToken_Failure(t *testing.T) {
	initError = nil
	provider := WorkloadIdentityTokenProvider{tokenCredential: MockTokenCredential{mockedToken: "mocked_token", mockedError: errors.New("Expected error from GetToken")}}
	scope := "https://management.core.windows.net/.default"

	token, err := provider.GetToken(scope)
	require.Error(t, err, "Expected error from GetToken")
	assert.Nil(t, token, "Expected token to be empty on error")
}

func TestGetToken_InitError(t *testing.T) {
	initError = errors.New("initialization error")
	provider := WorkloadIdentityTokenProvider{tokenCredential: MockTokenCredential{mockedToken: "mocked_token", mockedError: errors.New("Expected error from GetToken")}}

	token, err := provider.GetToken("https://management.core.windows.net/.default")
	require.Error(t, err, "Expected error from GetToken due to initialization error")
	assert.Nil(t, token, "Expected token to be empty on initialization error")
}

func TestCalculateCacheExpiryBasedOnTokenExpiry(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		expiry   time.Time
		expected time.Duration
		delta    float64
	}{
		{
			name:     "Future expiry (10min ahead)",
			expiry:   now.Add(10 * time.Minute),
			expected: 5 * time.Minute,
			delta:    10, // allow 10s difference
		},
		{
			name:     "Expiring in 5 minutes",
			expiry:   now.Add(5 * time.Second),
			expected: now.Sub(now.Add(5 * time.Minute)),
			delta:    10, // allow 10s difference
		},
		{
			name:     "Expires soon (4min ahead)",
			expiry:   now.Add(4 * time.Minute),
			expected: now.Sub(now.Add(1 * time.Minute)),
			delta:    10, // allow 10s difference
		},
		{
			name:     "Just expired (1s ago)",
			expiry:   now.Add(-1 * time.Second),
			expected: now.Sub(now.Add(5 * time.Minute)),
			delta:    10, // allow 10s difference
		},
		{
			name:     "Already expired (1m ago)",
			expiry:   now.Add(-1 * time.Minute),
			expected: now.Sub(now.Add(6 * time.Minute)),
			delta:    10, // allow 10s difference
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CalculateCacheExpiryBasedOnTokenExpiry(tt.expiry)
			if tt.delta > 0 {
				assert.InDelta(t, tt.expected.Seconds(), actual.Seconds(), tt.delta)
			} else {
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}
