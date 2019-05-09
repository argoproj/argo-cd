package fixtures

import (
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type Expectation func(c *Consequences) (done bool, message string)

func OperationPhaseIs(expected OperationPhase) Expectation {
	return func(c *Consequences) (done bool, message string) {
		actual := c.app().Status.OperationState.Phase
		return actual == expected, fmt.Sprintf("expect app %s's operation phase to be %s, is %s", c.context.name, expected, actual)
	}
}

func SyncStatusIs(expected SyncStatusCode) Expectation {
	return func(c *Consequences) (done bool, message string) {
		actual := c.app().Status.Sync.Status
		return actual == expected, fmt.Sprintf("expect app %s's sync status to be %s, is %s", c.context.name, expected, actual)
	}
}

func HealthIs(expected HealthStatusCode) Expectation {
	return func(c *Consequences) (bool, string) {
		actual := c.app().Status.Health.Status
		return actual == expected, fmt.Sprintf("expect app %s's health to be %s, is %s", c.context.name, expected, actual)
	}
}

func ResourceSyncStatusIs(resource string, expected SyncStatusCode) Expectation {
	return func(c *Consequences) (done bool, message string) {
		actual := c.resource(resource).Status
		return actual == expected, fmt.Sprintf("expect app %s's resource %s sync status to be %s, is %s", c.context.name, resource, expected, actual)
	}
}

func ResourceHealthIs(resource string, expected HealthStatusCode) Expectation {
	return func(c *Consequences) (done bool, message string) {
		actual := c.resource(resource).Health.Status
		return actual == expected, fmt.Sprintf("expect app %s's resource %s health to be %s, is %s", c.context.name, resource, expected, actual)
	}
}

func Deleted() Expectation {
	return func(c *Consequences) (bool, string) {
		_, err := c.Get()
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, ""
			}
			return true, err.Error()
		}
		return true, "not deleted"

	}
}

func App(predicate func(app *Application) bool, message string) Expectation {
	return func(c *Consequences) (bool, string) {
		return predicate(c.app()), message
	}
}

func Event(reason string, message string) Expectation {
	return func(c *Consequences) (bool, string) {
		list, err := c.context.fixture.KubeClientset.CoreV1().Events(c.context.fixture.ArgoCDNamespace).List(metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"involvedObject.name":      c.context.name,
				"involvedObject.namespace": c.context.fixture.ArgoCDNamespace,
			}).String(),
		})
		if err != nil {
			return true, err.Error()
		}

		for i := range list.Items {
			event := list.Items[i]
			if event.Reason == reason && strings.Contains(event.Message, message) {
				return true, ""
			}
		}
		return true, fmt.Sprintf("Unable to find event with reason=%s; message=%s", reason, message)
	}
}
