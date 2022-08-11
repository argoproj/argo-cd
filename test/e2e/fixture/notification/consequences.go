package notification

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) Services(block func(services *notification.Services, err error)) *Consequences {
	c.context.t.Helper()
	block(c.listServices())
	return c
}

func (c *Consequences) Triggers(block func(services *notification.Triggers, err error)) *Consequences {
	c.context.t.Helper()
	block(c.listTriggers())
	return c
}

func (c *Consequences) Templates(block func(services *notification.Templates, err error)) *Consequences {
	c.context.t.Helper()
	block(c.listTemplates())
	return c
}

func (c *Consequences) listServices() (*notification.Services, error) {
	_, notifClient, _ := fixture.ArgoCDClientset.NewNotificationClient()
	return notifClient.ListServices(context.Background(), &notification.ServicesListRequest{})
}

func (c *Consequences) listTriggers() (*notification.Triggers, error) {
	_, notifClient, _ := fixture.ArgoCDClientset.NewNotificationClient()
	return notifClient.ListTriggers(context.Background(), &notification.TriggersListRequest{})
}

func (c *Consequences) listTemplates() (*notification.Templates, error) {
	_, notifClient, _ := fixture.ArgoCDClientset.NewNotificationClient()
	return notifClient.ListTemplates(context.Background(), &notification.TemplatesListRequest{})
}

func (c *Consequences) When() *Actions {
	return c.actions
}

func (c *Consequences) Given() *Context {
	return c.context
}
