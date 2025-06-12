package repos

import (
	"log"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context      *Context
	lastOutput   string
	lastError    error
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

func (a *Actions) Create(args ...string) *Actions {
	args = a.prepareCreateArgs(args)

	//  are you adding new context values? if you only use them for this func, then use args instead
	a.runCli(args...)

	return a
}

func (a *Actions) prepareCreateArgs(args []string) []string {
	a.context.t.Helper()
	args = append([]string{
		"repo", "add", a.context.path,
	}, args...)
	if a.context.project != "" {
		args = append(args, "--project", a.context.project)
	}
	return args
}

func (a *Actions) Delete() *Actions {
	a.context.t.Helper()
	a.runCli("repo", "rm", a.context.path)
	return a
}

func (a *Actions) List() *Actions {
	a.context.t.Helper()
	a.runCli("repo", "list")
	return a
}

func (a *Actions) Get() *Actions {
	a.context.t.Helper()
	a.runCli("repo", "get", a.context.path)
	return a
}

func (a *Actions) Path(path string) *Actions {
	a.context.path = path
	return a
}

func (a *Actions) Project(project string) *Actions {
	a.context.project = project
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
	if !a.ignoreErrors && a.lastError != nil {
		log.Fatal(a.lastOutput)
	}
}
