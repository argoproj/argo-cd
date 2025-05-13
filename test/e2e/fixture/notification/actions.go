package notification

import (
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context *Context

	healthy bool
}

func (a *Actions) SetParamInNotificationConfigMap(key, value string) *Actions {
	a.context.t.Helper()
	require.NoError(a.context.t, fixture.SetParamInNotificationsConfigMap(key, value))
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Consequences{a.context, a}
}

func (a *Actions) Healthcheck() *Actions {
	a.context.t.Helper()
	_, err := fixture.DoHttpRequest("GET",
		"/metrics",
		fixture.GetNotificationServerAddress())
	a.healthy = err == nil
	return a
}
