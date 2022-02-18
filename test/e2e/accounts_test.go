package e2e

import (
	"context"
	"testing"

	"github.com/argoproj/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/session"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/account"
	"github.com/argoproj/argo-cd/v2/util/io"
)

func TestCreateAndUseAccount(t *testing.T) {
	ctx := accountFixture.Given(t)
	ctx.
		Name("test").
		When().
		Create().
		Then().
		And(func(account *account.Account, err error) {
			assert.Equal(t, account.Name, ctx.GetName())
			assert.Equal(t, account.Capabilities, []string{"login"})
		}).
		When().
		Login().
		Then().
		CurrentUser(func(user *session.GetUserInfoResponse, err error) {
			assert.Equal(t, user.LoggedIn, true)
			assert.Equal(t, user.Username, ctx.GetName())
		})
}

func TestCreateAndUseAccountCLI(t *testing.T) {
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
	testAccountClientset := headless.NewClientOrDie(&clientOpts, &cobra.Command{})

	closer, client := testAccountClientset.NewSessionClientOrDie()
	defer io.Close(closer)

	info, err := client.GetUserInfo(context.Background(), &session.GetUserInfoRequest{})
	assert.NoError(t, err)

	assert.Equal(t, info.Username, "test")
}

func TestLoginBadCredentials(t *testing.T) {
	EnsureCleanState(t)

	closer, sessionClient := ArgoCDClientset.NewSessionClientOrDie()
	defer io.Close(closer)

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
