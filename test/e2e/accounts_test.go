package e2e

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/util"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/errors"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
)

func TestCreateAndUseAccount(t *testing.T) {
	EnsureCleanState(t)

	output, err := RunCli("account", "list")
	errors.CheckError(err)

	assert.Equal(t, `NAME   ENABLED  CAPABILITIES
admin  true     login`, output)

	SetAccounts(map[string][]string{
		"test": {"login", "apiKey"},
	})

	output, err = RunCli("account", "list")
	errors.CheckError(err)

	assert.Equal(t, `NAME   ENABLED  CAPABILITIES
admin  true     login
test   true     login, apiKey`, output)

	token, err := RunCli("account", "generate-token", "--account", "test")
	errors.CheckError(err)

	clientOpts := ArgoCDClientset.ClientOptions()
	clientOpts.AuthToken = token
	testAccountClientset := argocdclient.NewClientOrDie(&clientOpts)

	closer, client := testAccountClientset.NewSessionClientOrDie()
	defer util.Close(closer)

	info, err := client.GetUserInfo(context.Background(), &session.GetUserInfoRequest{})
	assert.NoError(t, err)

	assert.Equal(t, info.Username, "test")
}
