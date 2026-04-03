package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSecretKey(t *testing.T) {
	secretName, tokenKey := ParseSecretKey("#my-secret:my-token")
	assert.Equal(t, "my-secret", secretName)
	assert.Equal(t, "$my-token", tokenKey)

	secretName, tokenKey = ParseSecretKey("#my-secret")
	assert.Equal(t, "argocd-secret", secretName)
	assert.Equal(t, "#my-secret", tokenKey)
}
