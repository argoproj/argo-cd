package fixtures

import (
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type state = string

const (
	failed    = "failed"
	pending   = "pending"
	succeeded = "succeeded"
)

type Expectation func(c *Consequences) (state state, message string)

func OperationPhaseIs(expected OperationPhase) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.app().Status.OperationState.Phase
		return simple(actual == expected, fmt.Sprintf("expect app %s's operation phase to be %s, is %s", c.context.name, expected, actual))
	}
}

func simple(success bool, message string) (state, string) {
	if success {
		return succeeded, ""
	} else {
		return pending, message
	}
}

func SyncStatusIs(expected SyncStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.app().Status.Sync.Status
		return simple(actual == expected, fmt.Sprintf("expect app %s's sync status to be %s, is %s", c.context.name, expected, actual))
	}
}

func Condition(conditionType ApplicationConditionType) Expectation {
	return func(c *Consequences) (state, string) {
		for _, condition := range c.app().Status.Conditions {
			if conditionType == condition.Type {
				return succeeded, ""
			}
		}
		return failed, "failed"
	}
}

func HealthIs(expected HealthStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.app().Status.Health.Status
		return simple(actual == expected, fmt.Sprintf("expect app %s's health to be %s, is %s", c.context.name, expected, actual))
	}
}

func ResourceSyncStatusIs(resource string, expected SyncStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.resource(resource).Status
		return simple(actual == expected, fmt.Sprintf("expect app %s's resource %s sync status to be %s, is %s", c.context.name, resource, expected, actual))
	}
}

func ResourceHealthIs(resource string, expected HealthStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.resource(resource).Health.Status
		return simple(actual == expected, fmt.Sprintf("expect app %s's resource %s health to be %s, is %s", c.context.name, resource, expected, actual))
	}
}

func DoesNotExist() Expectation {
	return func(c *Consequences) (state, string) {
		_, err := c.get()
		if err != nil {
			if apierrors.IsNotFound(err) {
				return succeeded, ""
			}
			return failed, err.Error()
		}
		return failed, "not deleted"
	}
}

func App(predicate func(app *Application) bool, message string) Expectation {
	return func(c *Consequences) (state, string) {
		return simple(predicate(c.app()), fmt.Sprintf(message))
	}
}

func Event(reason string, message string) Expectation {
	return func(c *Consequences) (state, string) {
		list, err := c.context.fixture.KubeClientset.CoreV1().Events(c.context.fixture.ArgoCDNamespace).List(metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"involvedObject.name":      c.context.name,
				"involvedObject.namespace": c.context.fixture.ArgoCDNamespace,
			}).String(),
		})
		if err != nil {
			return failed, err.Error()
		}

		for i := range list.Items {
			event := list.Items[i]
			if event.Reason == reason && strings.Contains(event.Message, message) {
				return succeeded, ""
			}
		}
		return failed, fmt.Sprintf("unable to find event with reason=%s; message=%s", reason, message)
	}
}
