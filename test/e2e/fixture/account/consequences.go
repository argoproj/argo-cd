package project

import (
	"context"
	"errors"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/session"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/io"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) And(ctx context.Context, block func(account *account.Account, err error)) *Consequences {
	c.context.t.Helper()
	block(c.get(ctx))
	return c
}

func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.t.Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}

func (c *Consequences) CurrentUser(ctx context.Context, block func(user *session.GetUserInfoResponse, err error)) *Consequences {
	c.context.t.Helper()
	block(c.getCurrentUser(ctx))
	return c
}

func (c *Consequences) get(ctx context.Context) (*account.Account, error) {
	_, accountClient, _ := fixture.ArgoCDClientset.NewAccountClient(ctx)
	accList, err := accountClient.ListAccounts(ctx, &account.ListAccountRequest{})
	if err != nil {
		return nil, err
	}
	for _, acc := range accList.Items {
		if acc.Name == c.context.name {
			return acc, nil
		}
	}
	return nil, errors.New("account not found")
}

func (c *Consequences) getCurrentUser(ctx context.Context) (*session.GetUserInfoResponse, error) {
	c.context.t.Helper()
	closer, client, err := fixture.ArgoCDClientset.NewSessionClient(ctx)
	require.NoError(c.context.t, err)
	defer io.Close(closer)
	return client.GetUserInfo(ctx, &session.GetUserInfoRequest{})
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return c.actions
}
