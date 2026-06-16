//go:build !darwin || (cgo && darwin)

package commands

import (
	"testing"

	"github.com/Azure/kubelogin/pkg/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAzureTokenOptions(t *testing.T) {
	t.Run("defaults LoginMethod to WorkloadIdentityLogin when no env var set", func(t *testing.T) {
		// given: no AAD_LOGIN_METHOD env var

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.Equal(t, token.WorkloadIdentityLogin, o.LoginMethod)
	})

	t.Run("uses default AAD server application ID when env var not set", func(t *testing.T) {
		// given: no AAD_SERVER_APPLICATION_ID env var

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.Equal(t, DEFAULT_AAD_SERVER_APPLICATION_ID, o.ServerID)
	})

	t.Run("overrides server application ID from AAD_SERVER_APPLICATION_ID", func(t *testing.T) {
		// given
		t.Setenv(envServerApplicationID, "custom-server-id")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.Equal(t, "custom-server-id", o.ServerID)
	})

	t.Run("overrides environment from AAD_ENVIRONMENT_NAME", func(t *testing.T) {
		// given
		t.Setenv(envEnvironmentName, "AzureUSGovernmentCloud")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.Equal(t, "AzureUSGovernmentCloud", o.Environment)
	})

	t.Run("does not enable PoP token when LoginMethod is not spn", func(t *testing.T) {
		// given
		t.Setenv(envIsPoPTokenEnabled, "true")
		t.Setenv(envPoPTokenClaims, "u=https://mycluster")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.Equal(t, token.WorkloadIdentityLogin, o.LoginMethod)
		assert.False(t, o.IsPoPTokenEnabled)
		assert.Empty(t, o.PoPTokenClaims)
	})

	t.Run("does not enable PoP token when AAD_IS_POP_TOKEN_ENABLED is absent", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.False(t, o.IsPoPTokenEnabled)
		assert.Empty(t, o.PoPTokenClaims)
	})

	t.Run("does not enable PoP token when AAD_IS_POP_TOKEN_ENABLED is false", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "false")
		t.Setenv(envPoPTokenClaims, "u=https://mycluster")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.False(t, o.IsPoPTokenEnabled)
		assert.Empty(t, o.PoPTokenClaims)
	})

	t.Run("enables PoP token and sets claims when LoginMethod is spn and both env vars set", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "true")
		t.Setenv(envPoPTokenClaims, "u=https://mycluster")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.True(t, o.IsPoPTokenEnabled)
		assert.Equal(t, "u=https://mycluster", o.PoPTokenClaims)
	})

	t.Run("returns error when AAD_IS_POP_TOKEN_ENABLED is true but AAD_POP_TOKEN_CLAIMS is absent", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "true")

		// when
		o, err := buildAzureTokenOptions()

		// then
		assert.Error(t, err)
		assert.Nil(t, o)
	})

	t.Run("returns error when AAD_IS_POP_TOKEN_ENABLED is true but AAD_POP_TOKEN_CLAIMS is empty string", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "true")
		t.Setenv(envPoPTokenClaims, "")

		// when
		o, err := buildAzureTokenOptions()

		// then
		assert.Error(t, err)
		assert.Nil(t, o)
	})

	t.Run("enables PoP token when AAD_IS_POP_TOKEN_ENABLED is True (mixed case)", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "True")
		t.Setenv(envPoPTokenClaims, "u=https://mycluster")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.True(t, o.IsPoPTokenEnabled)
		assert.Equal(t, "u=https://mycluster", o.PoPTokenClaims)
	})

	t.Run("enables PoP token when AAD_IS_POP_TOKEN_ENABLED is TRUE (uppercase)", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "TRUE")
		t.Setenv(envPoPTokenClaims, "u=https://mycluster")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.True(t, o.IsPoPTokenEnabled)
		assert.Equal(t, "u=https://mycluster", o.PoPTokenClaims)
	})

	t.Run("enables PoP token when AAD_IS_POP_TOKEN_ENABLED is 1", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "1")
		t.Setenv(envPoPTokenClaims, "u=https://mycluster")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.True(t, o.IsPoPTokenEnabled)
		assert.Equal(t, "u=https://mycluster", o.PoPTokenClaims)
	})

	t.Run("does not enable PoP token when AAD_IS_POP_TOKEN_ENABLED is an invalid value", func(t *testing.T) {
		// given
		t.Setenv("AAD_LOGIN_METHOD", token.ServicePrincipalLogin)
		t.Setenv(envIsPoPTokenEnabled, "garbage")
		t.Setenv(envPoPTokenClaims, "u=https://mycluster")

		// when
		o, err := buildAzureTokenOptions()

		// then
		require.NoError(t, err)
		assert.False(t, o.IsPoPTokenEnabled)
		assert.Empty(t, o.PoPTokenClaims)
	})
}
