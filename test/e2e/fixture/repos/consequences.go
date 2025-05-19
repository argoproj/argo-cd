package repos

import (
	"context"
	"fmt"

	repositorypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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

func (c *Consequences) And(block func(repository *v1alpha1.Repository, err error)) *Consequences {
	c.context.t.Helper()
	block(c.repo())
	return c
}

func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.t.Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}

func (c *Consequences) repo() (*v1alpha1.Repository, error) {
	app, err := c.get()
	return app, err
}

func (c *Consequences) get() (*v1alpha1.Repository, error) {
	_, repoClient, _ := fixture.ArgoCDClientset.NewRepoClient()

	repo, _ := repoClient.ListRepositories(context.Background(), &repositorypkg.RepoQuery{})
	for i := range repo.Items {
		if repo.Items[i].Repo == c.context.path {
			return repo.Items[i], nil
		}
	}

	return nil, fmt.Errorf("repo not found")
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	return c.actions
}
