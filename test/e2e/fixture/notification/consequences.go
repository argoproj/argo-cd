package notification

import (
	"context"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/notification"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) Services(ctx context.Context, block func(services *notification.ServiceList, err error)) *Consequences {
	c.context.t.Helper()
	block(c.listServices(ctx))
	return c
}

func (c *Consequences) Healthy(block func(healthy bool)) *Consequences {
	c.context.t.Helper()
	block(c.actions.healthy)
	return c
}

func (c *Consequences) Triggers(ctx context.Context, block func(services *notification.TriggerList, err error)) *Consequences {
	c.context.t.Helper()
	block(c.listTriggers(ctx))
	return c
}

func (c *Consequences) Templates(ctx context.Context, block func(services *notification.TemplateList, err error)) *Consequences {
	c.context.t.Helper()
	block(c.listTemplates(ctx))
	return c
}

func (c *Consequences) listServices(ctx context.Context) (*notification.ServiceList, error) {
	_, notifClient, _ := fixture.ArgoCDClientset.NewNotificationClient(ctx)
	return notifClient.ListServices(ctx, &notification.ServicesListRequest{})
}

func (c *Consequences) listTriggers(ctx context.Context) (*notification.TriggerList, error) {
	_, notifClient, _ := fixture.ArgoCDClientset.NewNotificationClient(ctx)
	return notifClient.ListTriggers(ctx, &notification.TriggersListRequest{})
}

func (c *Consequences) listTemplates(ctx context.Context) (*notification.TemplateList, error) {
	_, notifClient, _ := fixture.ArgoCDClientset.NewNotificationClient(ctx)
	return notifClient.ListTemplates(ctx, &notification.TemplatesListRequest{})
}

func (c *Consequences) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return c.actions
}

func (c *Consequences) Given() *Context {
	return c.context
}
