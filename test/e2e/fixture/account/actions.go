package account

import (
	"time"

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
		a.context.Name(): {"login"},
	}))
	_, _ = fixture.RunCli(a.prepareSetPasswordArgs(a.context.Name())...)
	return a
}

func (a *Actions) SetPermissions(permissions []fixture.ACL, roleName string) *Actions {
	a.context.T().Helper()
	require.NoError(a.context.T(), fixture.SetPermissions(permissions, a.context.Name(), roleName))
	return a
}

func (a *Actions) SetParamInSettingConfigMap(key, value string) *Actions {
	a.context.T().Helper()
	require.NoError(a.context.T(), fixture.SetParamInSettingConfigMap(key, value))
	return a
}

func (a *Actions) Login() *Actions {
	a.context.T().Helper()
	require.NoError(a.context.T(), fixture.LoginAs(a.context.Name()))
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
