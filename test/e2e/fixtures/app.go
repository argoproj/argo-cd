package fixtures

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type Context struct {
	fixture *Fixture
	t       *testing.T
	path    string
	name    string
}

func Given(f *Fixture, t *testing.T) *Context {
	f.EnsureCleanState()
	return &Context{f, t, "", ""}
}

func (c *Context) Path(path string) *Context {
	c.path = path
	return c
}

func (c *Context) Name(name string) *Context {
	c.name = name
	return c
}

type Actionable struct {
	context *Context
}

func (a *Actionable) Create() *Actionable {

	if a.context.name == "" {
		a.context.name = strings.ReplaceAll(a.context.path, "/", "-")
	}

	a.runCli("app", "create", a.context.name,
		"--repo", a.context.fixture.RepoURL(),
		"--path", a.context.path,
		"--dest-server", common.KubernetesInternalAPIServerAddr,
		"--dest-namespace", a.context.fixture.DeploymentNamespace)

	return a
}

func (c *Context) When() *Actionable {
	return &Actionable{c}
}

func (a *Actionable) Sync() *Actionable {
	return a.runCli("app", "sync", a.context.name, "--timeout", "5")
}

func (a *Actionable) TerminateOp() *Actionable {
	return a.runCli("app", "terminate-op", a.context.name)
}

func (a *Actionable) Patch(file string, jsonPath string) *Actionable {
	a.context.fixture.Patch(a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actionable) Delete() *Actionable {
	return a.runCli("app", "delete", a.context.name)
}

func (a *Actionable) runCli(args ...string) *Actionable {
	_, _ = a.context.fixture.RunCli(args...)
	return a
}

type Consequences struct {
	context    *Context
	actionable *Actionable
}

func (a *Actionable) Then() *Consequences {
	return &Consequences{a.context, a}
}

func (c *Consequences) Expect(e Expectation) *Consequences {
	WaitUntil(c.context.t, func() (done bool, err error) {
		done, message := e(c)
		if done {
			return true, nil
		} else {
			return false, errors.New(message)
		}
	})
	return c
}

func (c *Consequences) app() *Application {
	app, err := c.get()
	assert.NoError(c.context.t, err)
	return app
}
func (c *Consequences) get() (*Application, error) {
	return c.context.fixture.AppClientset.ArgoprojV1alpha1().Applications(c.context.fixture.ArgoCDNamespace).Get(c.context.name, v1.GetOptions{})
}

func (c *Consequences) resource(name string) ResourceStatus {
	for _, r := range c.app().Status.Resources {
		if r.Name == name {
			return r
		}
	}
	return ResourceStatus{
		Health: &HealthStatus{Status: HealthStatusUnknown},
	}
}

func (c *Consequences) Assert(block func(app *Application)) *Consequences {
	block(c.app())
	return c
}

func (c *Consequences) When() *Actionable {
	return c.actionable
}
