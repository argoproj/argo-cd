package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apiclient/session"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util"
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

func TestLoginBadCredentials(t *testing.T) {
	EnsureCleanState(t)

	closer, sessionClient := ArgoCDClientset.NewSessionClientOrDie()
	defer util.Close(closer)

	requests := []session.SessionCreateRequest{{
		Username: "user-does-not-exist", Password: "some-password",
	}, {
		Username: "admin", Password: "bad-password",
	}}

	for _, r := range requests {
		_, err := sessionClient.Create(context.Background(), &r)
		if !assert.Error(t, err) {
			return
		}
		errStatus, ok := status.FromError(err)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, codes.Unauthenticated, errStatus.Code())
		assert.Equal(t, "Invalid username or password", errStatus.Message())
	}
}
