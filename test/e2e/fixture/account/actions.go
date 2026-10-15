package account

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context    *Context
	lastOutput string
	lastError  error
}

func (a *Actions) prepareCanIGetLogsArgs() []string {
	a.context.T().Helper()
	return []string{
		"account", "can-i", "get", "logs", a.context.project + "/*",
	}
}

func (a *Actions) CanIGetLogs() *Actions {
	a.context.T().Helper()
	a.runCli(a.prepareCanIGetLogsArgs()...)
	return a
}

func (a *Actions) prepareSetPasswordArgs(account string) []string {
	a.context.T().Helper()
	return []string{
		"account", "update-password", "--account", account, "--current-password", fixture.AdminPassword, "--new-password", fixture.DefaultTestUserPassword,
	}
}

func (a *Actions) Create() *Actions {
	a.context.T().Helper()
	require.NoError(a.context.T(), fixture.SetAccounts(map[string][]string{
		a.context.GetName(): {"login"},
	}))
	_, _ = fixture.RunCli(a.prepareSetPasswordArgs(a.context.GetName())...)
	return a
}

func (a *Actions) SetPermissions(permissions []fixture.ACL, roleName string) *Actions {
	a.context.T().Helper()
	require.NoError(a.context.T(), fixture.SetPermissions(permissions, a.context.GetName(), roleName))
	return a
}

func (a *Actions) SetParamInSettingConfigMap(key, value string) *Actions {
	a.context.T().Helper()
	require.NoError(a.context.T(), fixture.SetParamInSettingConfigMap(key, value))
	return a
}

func (a *Actions) Login() *Actions {
	a.context.T().Helper()
	require.NoError(a.context.T(), fixture.LoginAs(a.context.GetName()))
	return a
}

func (a *Actions) CLILogin() *Actions {
	a.context.T().Helper()
	CLILogin(a.context.T(), a.context.GetName(), fixture.DefaultTestUserPassword, a.context.GetConfigPath())
	return a
}

func (a *Actions) SessionToken() *Actions {
	a.context.T().Helper()
	a.lastOutput, a.lastError = fixture.Run("", "../../dist/argocd", "account", "session-token", "--config", a.context.GetConfigPath())
	return a
}

func (a *Actions) SessionTokenJSON() *Actions {
	a.context.T().Helper()
	a.lastOutput, a.lastError = fixture.Run("", "../../dist/argocd", "account", "session-token", "-o", "json", "--config", a.context.GetConfigPath())
	return a
}

func (a *Actions) runCli(args ...string) {
	a.context.T().Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
}

func (a *Actions) Then() *Consequences {
	a.context.T().Helper()
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Consequences{a.context, a}
}

// CLILogin performs a CLI-based login using argocd login command with a custom config path.
// This properly establishes the context in the config file with server, user, and context information.
// Use this when you need to test CLI commands that rely on the local config file.
func CLILogin(t *testing.T, username, password, configPath string) {
	t.Helper()
	args := []string{
		"login", fixture.GetApiServerAddress(),
		"--username", username,
		"--password", password,
		"--config", configPath,
		"--insecure",
	}

	if fixture.IsPlainText() {
		args = append(args, "--plaintext")
	}

	loginOutput, err := fixture.Run("", "../../dist/argocd", args...)
	require.NoError(t, err, "Login should succeed")
	assert.Contains(t, loginOutput, "logged in successfully")
}
