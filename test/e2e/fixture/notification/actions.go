package notification

import "github.com/argoproj/argo-cd/v2/test/e2e/fixture"

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context *Context
}

func (a *Actions) SetParamInNotificationConfigMap(key, value string) *Actions {
	fixture.SetParamInNotificationsConfigMap(key, value)
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}
