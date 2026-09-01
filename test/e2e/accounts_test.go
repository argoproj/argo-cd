package e2e

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/account"
	"github.com/argoproj/argo-cd/v3/util/errors"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
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
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	testAccountClientset := headless.NewClientOrDie(&clientOpts, cmd)

	closer, client := testAccountClientset.NewSessionClientOrDie()
	defer utilio.Close(closer)

	info, err := client.GetUserInfo(t.Context(), &session.GetUserInfoRequest{})
	require.NoError(t, err)

	assert.Equal(t, "test", info.Username)
}

func waitForLoginStatus(t *testing.T, username, password string, expectBlocked bool) {
	t.Helper()
	var lastErr error
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		closer, sessionClient := ArgoCDClientset.NewSessionClientOrDie()
		_, err := sessionClient.Create(context.Background(), &session.SessionCreateRequest{
			Username: username,
			Password: password,
		})
		utilio.Close(closer)
		lastErr = err
		if expectBlocked {
			if st, ok := status.FromError(err); ok && st.Code() == codes.PermissionDenied {
				return
			}
		} else if err == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	if expectBlocked {
		t.Fatalf("expected login to be blocked with PermissionDenied within 15s, last result: %v", lastErr)
	}

	t.Fatalf("expected login to succeed within 15s, last error: %v", lastErr)
}

func createTestUser(t *testing.T) {
	t.Helper()
	require.NoError(t, SetAccounts(map[string][]string{"test": {"login"}}))
	_, err := RunCli("account", "update-password",
		"--account", "test",
		"--current-password", AdminPassword,
		"--new-password", DefaultTestUserPassword,
	)
	require.NoError(t, err)
}

// TestPreventLoginFlagDisabledPermitsLoginWithNoPermissions verifies that when the feature flag is
// off (the default), a user with no RBAC permissions can still log in.
func TestPreventLoginFlagDisabledPermitsLoginWithNoPermissions(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)

	closer, sessionClient := ArgoCDClientset.NewSessionClientOrDie()
	defer utilio.Close(closer)
	_, err := sessionClient.Create(context.Background(), &session.SessionCreateRequest{
		Username: "test",
		Password: DefaultTestUserPassword,
	})
	require.NoError(t, err)
}

// TestPreventLoginFlagBlocksLoginWithNoPermissions verifies that when the feature flag is enabled,
// a user with no RBAC permissions is denied login with PermissionDenied.
func TestPreventLoginFlagBlocksLoginWithNoPermissions(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, true)
}

// TestPreventLoginFlagPermitsLoginWithPermissions verifies that when the feature flag is enabled,
// a user who has at least one allow rule is not blocked.
func TestPreventLoginFlagPermitsLoginWithPermissions(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	require.NoError(t, SetPermissions([]ACL{{Resource: "applications", Action: "get", Scope: "*/*"}}, "test", "test-role"))
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, false)
}

// TestPreventLoginDenyOverrideBlocksLogin verifies that a user whose allowed rules are all
// overridden by a denied rule is still blocked — regression test for the HasAnyAllowPermission fix.
func TestPreventLoginDenyOverrideBlocksLogin(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	require.NoError(t, SetRawPolicyCsv("p, test, applications, get, *, allow\np, test, *, *, *, deny"))
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, true)
}

// TestPreventLoginFlagToggleUnblocksLogin verifies that disabling the flag at runtime (without a
// server restart) allows a previously blocked user to log in again.
func TestPreventLoginFlagToggleUnblocksLogin(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, true)

	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "false"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, false)
}

func bindTestUserToProjectRole(t *testing.T, roleName string, policies ...string) {
	t.Helper()
	require.NoError(t, SetProjectRoles("default", v1alpha1.ProjectRole{
		Name:     roleName,
		Policies: policies,
		Groups:   []string{"test"},
	}))
}

func TestPreventLoginProjectRolePermitsLogin(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	bindTestUserToProjectRole(t, "developer", "p, proj:default:developer, applications, sync, default/*, allow")
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, false)
}

func TestPreventLoginProjectRoleWithOnlyDenyPermitsLogin(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	bindTestUserToProjectRole(t, "deny-only", "p, proj:default:deny-only, applications, sync, default/*, deny")
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, false)
}

func TestPreventLoginProjectRoleCancelledByDenyBlocksLogin(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	bindTestUserToProjectRole(t, "developer", "p, proj:default:developer, applications, sync, default/*, allow")
	require.NoError(t, SetRawPolicyCsv("p, test, *, *, *, deny"))
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, true)
}

func TestPreventLoginProjectRoleGrantUnblocksLogin(t *testing.T) {
	EnsureCleanState(t)
	createTestUser(t)
	require.NoError(t, SetParamInRBACConfigMap("policy.prevent-login-without-permissions", "true"))

	waitForLoginStatus(t, "test", DefaultTestUserPassword, true)

	bindTestUserToProjectRole(t, "developer", "p, proj:default:developer, applications, sync, default/*, allow")

	waitForLoginStatus(t, "test", DefaultTestUserPassword, false)
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

func TestAccountSessionToken(t *testing.T) {
	// Create a unique temporary config directory for this test to support parallel execution
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	ctx := accountFixture.Given(t)
	ctx.
		Name("test").
		ConfigPath(configPath).
		When().
		Create().
		CLILogin().
		SessionToken().
		Then().
		AndCLIOutput(func(output string, err error) {
			require.NoError(t, err, "session-token command should succeed")

			// Verify token is not empty and is a valid JWT format
			token := strings.TrimSpace(output)
			assert.NotEmpty(t, token)
			assert.True(t, jwtutil.IsValid(token), "Token should be a valid JWT format")

			// Verify the token can be used for authentication
			clientOpts := ArgoCDClientset.ClientOptions()
			clientOpts.AuthToken = token
			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())
			testAccountClientset := headless.NewClientOrDie(&clientOpts, cmd)

			closer, client := testAccountClientset.NewSessionClientOrDie()
			defer utilio.Close(closer)

			info, err := client.GetUserInfo(t.Context(), &session.GetUserInfoRequest{})
			require.NoError(t, err)

			// Verify the token belongs to the test user
			assert.Equal(t, ctx.GetName(), info.Username)
			assert.True(t, info.LoggedIn)
		}).
		When().
		SessionTokenJSON().
		Then().
		AndCLIOutput(func(output string, err error) {
			require.NoError(t, err, "session-token with JSON output should succeed")

			// Parse JSON output
			var tokenInfo map[string]any
			err = json.Unmarshal([]byte(output), &tokenInfo)
			require.NoError(t, err, "JSON output should be valid")

			// Verify JSON fields exist
			assert.Contains(t, tokenInfo, "token")
			assert.Contains(t, tokenInfo, "type")
			assert.Contains(t, tokenInfo, "issuer")
			assert.Contains(t, tokenInfo, "username")
			assert.Contains(t, tokenInfo, "has_refresh_token")

			// Verify token type is local
			assert.Equal(t, "local", tokenInfo["type"])

			// Verify username
			username, ok := tokenInfo["username"].(string)
			require.True(t, ok, "username should be a string")
			assert.Equal(t, ctx.GetName(), username)
		})
}
