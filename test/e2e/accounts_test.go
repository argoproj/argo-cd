package e2e

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/account"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/io"
)

func TestCreateAndUseAccount(t *testing.T) {
	ctx := accountFixture.Given(t)
	ctx.
		Name("test").
		When().
		Create().
		Then().
		And(func(account *account.Account, _ error) {
			assert.Equal(t, account.Name, ctx.GetName())
			assert.Equal(t, []string{"login"}, account.Capabilities)
		}).
		When().
		Login().
		Then().
		CurrentUser(func(user *session.GetUserInfoResponse, _ error) {
			assert.True(t, user.LoggedIn)
			assert.Equal(t, user.Username, ctx.GetName())
		})
}

func TestCanIGetLogsAllow(t *testing.T) {
	ctx := accountFixture.Given(t)
	ctx.
		Name("test").
		Project(ProjectName).
		When().
		Create().
		Login().
		SetPermissions([]ACL{
			{
				Resource: "logs",
				Action:   "get",
				Scope:    ProjectName + "/*",
			},
			{
				Resource: "apps",
				Action:   "get",
				Scope:    ProjectName + "/*",
			},
		}, "log-viewer").
		CanIGetLogs().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Contains(t, output, "yes")
		})
}

func TestCanIGetLogsDeny(t *testing.T) {
	ctx := accountFixture.Given(t)
	ctx.
		Name("test").
		When().
		Create().
		Login().
		CanIGetLogs().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Contains(t, output, "no")
		})
}

func TestCreateAndUseAccountCLI(t *testing.T) {
	EnsureCleanState(t)

	output, err := RunCli("account", "list")
	errors.CheckError(err)

	assert.Equal(t, `NAME   ENABLED  CAPABILITIES
admin  true     login`, output)

	errors.CheckError(SetAccounts(map[string][]string{
		"test": {"login", "apiKey"},
	}))

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

	info, err := client.GetUserInfo(t.Context(), &session.GetUserInfoRequest{})
	require.NoError(t, err)

	assert.Equal(t, "test", info.Username)
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
		_, err := sessionClient.Create(t.Context(), &r)
		require.Error(t, err)
		errStatus, ok := status.FromError(err)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, codes.Unauthenticated, errStatus.Code())
		assert.Equal(t, "Invalid username or password", errStatus.Message())
	}
}
