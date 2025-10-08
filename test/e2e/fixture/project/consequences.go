package project

import (
	"context"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/project"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) Expect() *Consequences {
	return c
}

func (c *Consequences) And(block func(app *project.DetailedProjectsResponse, err error)) *Consequences {
	c.context.t.Helper()
	block(c.detailedProject())
	return c
}

func (c *Consequences) detailedProject() (*project.DetailedProjectsResponse, error) {
	prj, err := c.get(c.context.t.Context())
	return prj, err
}

func (c *Consequences) get(ctx context.Context) (*project.DetailedProjectsResponse, error) {
	_, projectClient, _ := fixture.ArgoCDClientset.NewProjectClient(ctx)
	prj, err := projectClient.GetDetailedProject(ctx, &project.ProjectQuery{
		Name: c.context.name,
	})

	return prj, err
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return c.actions
}
