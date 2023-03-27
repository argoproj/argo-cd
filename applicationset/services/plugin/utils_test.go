package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceStringSecret(t *testing.T) {
	secretValues := map[string]string{"my-secret-key": "my-secret-value"}
	result := ReplaceStringSecret("$my-secret-key", secretValues)
	assert.Equal(t, "my-secret-value", result)

	result = ReplaceStringSecret("$invalid-secret-key", secretValues)
	assert.Equal(t, "$invalid-secret-key", result)

	result = ReplaceStringSecret("", secretValues)
	assert.Equal(t, "", result)

	result = ReplaceStringSecret("my-value", secretValues)
	assert.Equal(t, "my-value", result)
}

func TestParseSecretKey(t *testing.T) {
	secretName, tokenKey := ParseSecretKey("#my-secret:my-token")
	assert.Equal(t, "my-secret", secretName)
	assert.Equal(t, "$my-token", tokenKey)

	secretName, tokenKey = ParseSecretKey("#my-secret")
	assert.Equal(t, "argocd-secret", secretName)
	assert.Equal(t, "#my-secret", tokenKey)
}
