package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/common"
)

func TestGetAccounts_NoAccountsConfigured(t *testing.T) {
	_, settingsManager := fixtures(nil)
	accounts, err := settingsManager.GetAccounts()
	assert.NoError(t, err)

	_, ok := accounts[common.ArgoCDAdminUsername]
	assert.True(t, ok)
}

func TestGetAccounts_HasConfiguredAccounts(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{"accounts.test": "apiKey"}, func(secret *v1.Secret) {
		secret.Data["accounts.test.tokens"] = []byte(`[{"id":"123","iat":1583789194,"exp":1583789194}]`)
	})
	accounts, err := settingsManager.GetAccounts()
	assert.NoError(t, err)

	acc, ok := accounts["test"]
	assert.True(t, ok)
	assert.ElementsMatch(t, []AccountCapability{AccountCapabilityApiKey}, acc.Capabilities)
	assert.ElementsMatch(t, []Token{{ID: "123", IssuedAt: 1583789194, ExpiresAt: 1583789194}}, acc.Tokens)
	assert.True(t, acc.Enabled)
}

func TestGetAccounts_DisableAccount(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"accounts.test":         "apiKey",
		"accounts.test.enabled": "false",
	})
	accounts, err := settingsManager.GetAccounts()
	assert.NoError(t, err)

	acc, ok := accounts["test"]
	assert.True(t, ok)
	assert.False(t, acc.Enabled)
}

func TestGetAccount(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"accounts.test": "apiKey",
	})

	t.Run("ExistingUserName", func(t *testing.T) {
		_, err := settingsManager.GetAccount("test")

		assert.NoError(t, err)
	})

	t.Run("IncorrectName", func(t *testing.T) {
		_, err := settingsManager.GetAccount("incorrect-name")

		assert.Error(t, err)
		assert.Equal(t, status.Code(err), codes.NotFound)
	})
}
