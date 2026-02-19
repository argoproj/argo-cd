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
	utilio "github.com/argoproj/argo-cd/v3/util/io"
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

func TestCanIGetLogs(t *testing.T) {
	tests := []struct {
		name         string
		policies     []ACL
		queryScope   string
		expectedResp string
	}{
		{
			name: "Policies and query without namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + "/*"},
			},
			queryScope:   ProjectName + "/*",
			expectedResp: "yes",
		},
		{
			name: "Query without namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + TestNamespace() + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + TestNamespace() + "/*"},
			},
			queryScope:   ProjectName + "/*",
			expectedResp: "yes",
		},
		{
			name: "Policies without namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + "/*"},
			},
			queryScope:   ProjectName + TestNamespace() + "/*",
			expectedResp: "yes",
		},
		{
			name: "Both policies and query with namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + TestNamespace() + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + TestNamespace() + "/*"},
			},
			queryScope:   ProjectName + TestNamespace() + "/*",
			expectedResp: "yes",
		},
		{
			name: "Both policies and query with other namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + AppNamespace() + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + AppNamespace() + "/*"},
			},
			queryScope:   ProjectName + AppNamespace() + "/*",
			expectedResp: "yes",
		},
		{
			name: "Policies with wildcard and query without namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + "/*/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + "/*/*"},
			},
			queryScope:   ProjectName + "/*",
			expectedResp: "yes",
		},
		{
			name: "Policies with wildcard and query with other namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + "/*/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + "/*/*"},
			},
			queryScope:   ProjectName + AppNamespace() + "/*",
			expectedResp: "yes",
		},
		{
			name: "Policies with wildcard and query with namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + "/*/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + "/*/*"},
			},
			queryScope:   ProjectName + TestNamespace() + "/*",
			expectedResp: "yes",
		},
		{
			name:         "No policies",
			policies:     []ACL{},
			queryScope:   ProjectName + "/*",
			expectedResp: "no",
		},
		{
			name: "Policies with other namespace and query without namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + AppNamespace() + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + AppNamespace() + "/*"},
			},
			queryScope:   ProjectName + "/*",
			expectedResp: "no",
		},
		{
			name: "Policies with default namespace and query with other namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + AppNamespace() + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + AppNamespace() + "/*"},
			},
			queryScope:   ProjectName + TestNamespace() + "/*",
			expectedResp: "no",
		},
		{
			name: "Policies with other namespace and query with default namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + TestNamespace() + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + TestNamespace() + "/*"},
			},
			queryScope:   ProjectName + AppNamespace() + "/*",
			expectedResp: "no",
		},
		{
			name: "Policies without namespace and query with other namespace",
			policies: []ACL{
				{Resource: "logs", Action: "get", Scope: ProjectName + "/*"},
				{Resource: "apps", Action: "get", Scope: ProjectName + "/*"},
			},
			queryScope:   ProjectName + AppNamespace() + "/*",
			expectedResp: "no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := accountFixture.Given(t)
			ctx.
				Name("test")
			ctx.
				When().
				Create().
				Login().
				SetPermissions(tt.policies, "log-viewer").
				CanIGetLogs(tt.queryScope).
				Then().
				AndCLIOutput(func(output string, _ error) {
					assert.Contains(t, output, tt.expectedResp)
				})
		})
	}
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
	defer utilio.Close(closer)

	info, err := client.GetUserInfo(t.Context(), &session.GetUserInfoRequest{})
	require.NoError(t, err)

	assert.Equal(t, "test", info.Username)
}

func TestLoginBadCredentials(t *testing.T) {
	EnsureCleanState(t)

	closer, sessionClient := ArgoCDClientset.NewSessionClientOrDie()
	defer utilio.Close(closer)

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
