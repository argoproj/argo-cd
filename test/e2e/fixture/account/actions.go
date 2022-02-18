package project

import (
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context      *Context
	ignoreErrors bool
}

func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
	return a
}

func (a *Actions) prepareSetPasswordArgs(account string) []string {
	a.context.t.Helper()
	return []string{
		"account", "update-password", "--account", account, "--current-password", fixture.AdminPassword, "--new-password", fixture.DefaultTestUserPassword, "--plaintext",
	}
}

func (a *Actions) Create() *Actions {
	fixture.SetAccounts(map[string][]string{
		a.context.name: {"login"},
	})
	_, _ = fixture.RunCli(a.prepareSetPasswordArgs(a.context.name)...)
	return a
}

func (a *Actions) SetPermissions(permissions []fixture.ACL, roleName string) *Actions {
	fixture.SetPermissions(permissions, a.context.name, roleName)
	return a
}

func (a *Actions) Login() *Actions {
	fixture.LoginAs(a.context.name)
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}
