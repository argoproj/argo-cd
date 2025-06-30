package admin

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
	lastOutput   string
	lastError    error
}

func (a *Actions) prepareExportCommand() []string {
	a.context.t.Helper()
	args := []string{"export", "--application-namespaces", fixture.AppNamespace()}

	return args
}

func (a *Actions) prepareImportCommand() []string {
	a.context.t.Helper()
	args := []string{"import", "--application-namespaces", fixture.AppNamespace(), "-"}

	return args
}

func (a *Actions) RunExport() *Actions {
	a.context.t.Helper()
	a.runCli(a.prepareExportCommand()...)
	return a
}

func (a *Actions) RunImport(stdin string) *Actions {
	a.context.t.Helper()
	a.runCliWithStdin(stdin, a.prepareImportCommand()...)
	return a
}

func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
	return a
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = RunCli(args...)
}

func (a *Actions) runCliWithStdin(stdin string, args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = RunCliWithStdin(stdin, args...)
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}
