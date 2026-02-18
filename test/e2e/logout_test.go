package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"

	sessionFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/session"
)

// TestLogoutRevokesToken verifies that after logging out via the /auth/logout endpoint,
// the JWT token is revoked and can no longer be used for API calls.
func TestLogoutRevokesToken(t *testing.T) {
	sessionFixture.Given(t).
		When().
		Login("token1").
		GetUserInfo("token1").
		ListAccounts("token1").
		Then().
		ActionShouldSucceed().
		When().
		Logout("token1").
		GetUserInfo("token1").
		ListAccounts("token1").
		Then().
		ActionShouldFail(func(err error) {
			require.Contains(t, err.Error(), "token is revoked")
		})
}

// TestLogoutDoesNotAffectOtherSessions verifies that revoking one session token
// does not invalidate a different session token for the same user.
func TestLogoutDoesNotAffectOtherSessions(t *testing.T) {
	sessionFixture.Given(t).
		When().
		Login("token1").
		Login("token2").
		GetUserInfo("token1").
		GetUserInfo("token2").
		ListAccounts("token1").
		Then().
		ActionShouldSucceed().
		When().
		GetUserInfo("token2").
		ListAccounts("token2").
		Then().
		ActionShouldSucceed().
		When().
		Logout("token1").
		GetUserInfo("token1").
		ListApplications("token1").
		Then().
		ActionShouldFail(func(err error) {
			require.Contains(t, err.Error(), "token is revoked")
		}).
		When().
		ListApplications("token2").
		Then().
		ActionShouldSucceed()
}

// TestLogoutRevokedTokenCannotAccessAPIs verifies that a revoked token is rejected
// across different API endpoints, not just GetUserInfo.
func TestLogoutRevokedTokenCannotAccessAPIs(t *testing.T) {
	sessionFixture.Given(t).
		When().
		Login("token1").
		GetUserInfo("token1").
		Logout("token1").
		GetUserInfo("token1").
		ListAccounts("token1").
		Then().
		ActionShouldFail(func(err error) {
			require.Contains(t, err.Error(), "token is revoked")
		}).
		// Project API
		When().
		ListProjects("token1").
		Then().
		ActionShouldFail(func(err error) {
			require.Contains(t, err.Error(), "token is revoked")
		}).
		// Repository API
		When().
		ListRepositories("token1").
		Then().
		ActionShouldFail(func(err error) {
			require.Contains(t, err.Error(), "token is revoked")
		}).
		// Application API
		When().
		ListApplications("token1").
		Then().
		ActionShouldFail(func(err error) {
			require.Contains(t, err.Error(), "token is revoked")
		})
}
