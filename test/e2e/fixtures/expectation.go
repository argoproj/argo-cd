package fixtures

import (
	"fmt"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type Expectation func(a *App) (done bool, message string)

func SyncStatusIs(expected SyncStatusCode) Expectation {
	return func(a *App) (done bool, message string) {
		actual := a.get().Status.Sync.Status
		return actual == expected, fmt.Sprintf("expect app %s's sync status to be %s, is %s", a.Name, expected, actual)
	}
}

func HealthIs(expected HealthStatusCode) Expectation {
	return func(a *App) (bool, string) {
		actual := a.get().Status.Health.Status
		return actual == expected, fmt.Sprintf("expect app %s's health to be %s, is %s", a.Name, expected, actual)
	}
}

func ResourceSyncStatusIs(resource string, expected SyncStatusCode) Expectation {
	return func(a *App) (done bool, message string) {
		actual := a.resource(resource).Status
		return actual == expected, fmt.Sprintf("expect app %s's resource %s sync status to be %s, is %s", a.Name, resource, expected, actual)
	}
}

func ResourceHealthIs(resource string, expected HealthStatusCode) Expectation {
	return func(a *App) (done bool, message string) {
		actual := a.resource(resource).Health.Status
		return actual == expected, fmt.Sprintf("expect app %s's resource %s health to be %s, is %s", a.Name, resource, expected, actual)
	}
}
