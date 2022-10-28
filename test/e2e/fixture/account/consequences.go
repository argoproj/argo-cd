package project

import (
	"context"
	"errors"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/session"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) And(block func(account *account.Account, err error)) *Consequences {
	c.context.t.Helper()
	block(c.get())
	return c
}

func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.t.Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}

func (c *Consequences) CurrentUser(block func(user *session.GetUserInfoResponse, err error)) *Consequences {
	c.context.t.Helper()
	block(c.getCurrentUser())
	return c
}

func (c *Consequences) get() (*account.Account, error) {
	_, accountClient, _ := fixture.ArgoCDClientset.NewAccountClient()
	accList, err := accountClient.ListAccounts(context.Background(), &account.ListAccountRequest{})
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

func (c *Consequences) getCurrentUser() (*session.GetUserInfoResponse, error) {
	closer, client, err := fixture.ArgoCDClientset.NewSessionClient()
	CheckError(err)
	defer io.Close(closer)
	return client.GetUserInfo(context.Background(), &session.GetUserInfoRequest{})
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	return c.actions
}
