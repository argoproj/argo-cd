package admin

import (
	"time"

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

func (a *Actions) prepareExportCommand() []string {
	a.context.T().Helper()
	args := []string{"export", "--application-namespaces", fixture.AppNamespace()}

	return args
}

func (a *Actions) prepareImportCommand() []string {
	a.context.T().Helper()
	args := []string{"import", "--application-namespaces", fixture.AppNamespace(), "-"}

	return args
}

func (a *Actions) RunExport() *Actions {
	a.context.T().Helper()
	a.runCli(a.prepareExportCommand()...)
	return a
}

func (a *Actions) RunImport(stdin string) *Actions {
	a.context.T().Helper()
	a.runCliWithStdin(stdin, a.prepareImportCommand()...)
	return a
}

func (a *Actions) runCli(args ...string) {
	a.context.T().Helper()
	a.lastOutput, a.lastError = RunCli(args...)
}

func (a *Actions) runCliWithStdin(stdin string, args ...string) {
	a.context.T().Helper()
	a.lastOutput, a.lastError = RunCliWithStdin(stdin, args...)
}

func (a *Actions) Then() *Consequences {
	a.context.T().Helper()
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Consequences{a.context, a}
}
