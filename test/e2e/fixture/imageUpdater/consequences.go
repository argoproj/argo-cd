package imageUpdater

import (
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/stretchr/testify/require"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (a *Actions) And(block func()) *Actions {
	a.context.t.Helper()
	block()
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
	if !a.ignoreErrors {
		require.Empty(a.context.t, a.lastError)
	}
}
