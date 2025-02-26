package project

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/project"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
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
	prj, err := c.get()
	return prj, err
}

func (c *Consequences) get() (*project.DetailedProjectsResponse, error) {
	_, projectClient, _ := fixture.ArgoCDClientset.NewProjectClient()
	prj, err := projectClient.GetDetailedProject(context.Background(), &project.ProjectQuery{
		Name: c.context.name,
	})

	return prj, err
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	return c.actions
}
