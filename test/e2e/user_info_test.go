package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
)

func TestUserInfo(t *testing.T) {
	EnsureCleanState(t)

	output, err := RunCli("account", "get-user-info")

	assert.NoError(t, err)
	assert.Equal(t, `Logged In: true
Username: admin
Issuer: argocd
Groups: `, output)
}
