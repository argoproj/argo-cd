package notification

import (
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/errors"
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
	errors.CheckError(fixture.SetParamInNotificationsConfigMap(key, value))
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
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
