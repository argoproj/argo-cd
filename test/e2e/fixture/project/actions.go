package project

import (
	"context"
	"strings"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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

func (a *Actions) AddDestination(cluster string, namespace string) *Actions {
	a.runCli("proj", "add-destination", a.context.name, cluster, namespace)
	return a
}

func (a *Actions) AddSource(repo string) *Actions {
	a.runCli("proj", "add-source", a.context.name, repo)
	return a
}

func (a *Actions) UpdateProject(updater func(project *v1alpha1.AppProject)) *Actions {
	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.TestNamespace()).Get(context.TODO(), a.context.name, v1.GetOptions{})
	require.NoError(a.context.t, err)
	updater(proj)
	_, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.TestNamespace()).Update(context.TODO(), proj, v1.UpdateOptions{})
	require.NoError(a.context.t, err)
	return a
}

func (a *Actions) Name(name string) *Actions {
	a.context.name = name
	return a
}

func (a *Actions) prepareCreateArgs(args []string) []string {
	a.context.t.Helper()
	args = append([]string{
		"proj", "create", a.context.name,
	}, args...)

	if a.context.destination != "" {
		args = append(args, "--dest", a.context.destination)
	}

	if len(a.context.sourceNamespaces) > 0 {
		args = append(args, "--source-namespaces", strings.Join(a.context.sourceNamespaces, ","))
	}
	return args
}

func (a *Actions) Delete() *Actions {
	a.context.t.Helper()
	a.runCli("proj", "delete", a.context.name)
	return a
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
