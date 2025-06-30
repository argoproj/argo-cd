package admin

import (
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/admin/utils"
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
func (c *Consequences) AndExportedResources(block func(resources *ExportedResources, err error)) {
	result, err := GetExportedResourcesFromOutput(c.actions.lastOutput)
	block(&result, err)
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	return c.actions
}
