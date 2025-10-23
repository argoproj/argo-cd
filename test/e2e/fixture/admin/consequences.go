package admin

import (
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/admin/utils"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) And(block func()) *Consequences {
	c.context.t.Helper()
	block()
	return c
}

func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.t.Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}

// For use after running export with the exported resources desirialized
func (c *Consequences) AndExportedResources(block func(resources *utils.ExportedResources, err error)) {
	result, err := utils.GetExportedResourcesFromOutput(c.actions.lastOutput)
	block(&result, err)
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return c.actions
}
