package fixtures

import (
	"fmt"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type Expectation func(a *App) (done bool, message string)

func SyncStatusIs(expected SyncStatusCode) Expectation {
	return func(a *App) (done bool, message string) {
		actual := a.get().Status.Sync.Status
		return actual == expected, fmt.Sprintf("app %s's sync status to be %s, is %s", a.Name, actual, expected)
	}
}

func HealthIs(expected HealthStatusCode) Expectation {
	return func(a *App) (bool, string) {
		actual := a.get().Status.Health.Status
		return actual == expected, fmt.Sprintf("app %s's health to be %s, is %s", a.Name, actual, expected)
	}
}

func ResourceSyncStatusIs(name string, expected SyncStatusCode) Expectation {
	return func(a *App) (done bool, message string) {
		actual := a.resource(name).Status
		return actual == expected, fmt.Sprintf("app %s's resource %s sync status to be %s, is %s", a.Name, name, actual, expected)
	}
}

func ResourceHealthIs(name string, expected HealthStatusCode) Expectation {
	return func(a *App) (done bool, message string) {
		actual := a.resource(name).Health.Status
		return actual == expected, fmt.Sprintf("app %s's resource %s health to be %s, is %s", a.Name, name, actual, expected)
	}
}
