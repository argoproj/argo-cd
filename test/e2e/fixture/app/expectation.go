package app

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
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
		operationState := c.app().Status.OperationState
		actual := OperationRunning
		if operationState != nil {
			actual = operationState.Phase
		}
		return simple(actual == expected, fmt.Sprintf("operation phase should be %s, is %s", expected, actual))
	}
}

func simple(success bool, message string) (state, string) {
	if success {
		return succeeded, message
	} else {
		return pending, message
	}
}

func SyncStatusIs(expected SyncStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.app().Status.Sync.Status
		return simple(actual == expected, fmt.Sprintf("sync status to be %s, is %s", expected, actual))
	}
}

func Condition(conditionType ApplicationConditionType) Expectation {
	return func(c *Consequences) (state, string) {
		message := fmt.Sprintf("condition of type %s", conditionType)
		for _, condition := range c.app().Status.Conditions {
			if conditionType == condition.Type {
				return succeeded, message
			}
		}
		return failed, message
	}
}

func HealthIs(expected HealthStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.app().Status.Health.Status
		return simple(actual == expected, fmt.Sprintf("health to should %s, is %s", expected, actual))
	}
}

func ResourceSyncStatusIs(kind, resource string, expected SyncStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.resource(kind, resource).Status
		return simple(actual == expected, fmt.Sprintf("resource '%s/%s' sync status should be %s, is %s", kind, resource, expected, actual))
	}
}

func ResourceHealthIs(kind, resource string, expected HealthStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.resource(kind, resource).Health.Status
		return simple(actual == expected, fmt.Sprintf("resource '%s/%s' health should be %s, is %s", kind, resource, expected, actual))
	}
}

func DoesNotExist() Expectation {
	return func(c *Consequences) (state, string) {
		_, err := c.get()
		if err != nil {
			if apierr.IsNotFound(err) {
				return succeeded, "app does not exist"
			}
			return failed, err.Error()
		}
		return pending, "app should not exist"
	}
}

func Pod(predicate func(p v1.Pod) bool) Expectation {
	return func(c *Consequences) (state, string) {
		pods, err := pods()
		if err != nil {
			return failed, err.Error()
		}
		for _, pod := range pods.Items {
			if predicate(pod) {
				return succeeded, fmt.Sprintf("pod predicate matched pod named '%s'", pod.GetName())
			}
		}
		return pending, fmt.Sprintf("pod predicate does not match pods")
	}
}

func NotPod(predicate func(p v1.Pod) bool) Expectation {
	return func(c *Consequences) (state, string) {
		pods, err := pods()
		if err != nil {
			return failed, err.Error()
		}
		for _, pod := range pods.Items {
			if predicate(pod) {
				return pending, fmt.Sprintf("pod predicate matched pod named '%s'", pod.GetName())
			}
		}
		return succeeded, fmt.Sprintf("pod predicate did not match any pod")
	}
}

func pods() (*v1.PodList, error) {
	fixture.KubeClientset.CoreV1()
	pods, err := fixture.KubeClientset.CoreV1().Pods(fixture.DeploymentNamespace()).List(metav1.ListOptions{})
	return pods, err
}

func Event(reason string, message string) Expectation {
	return func(c *Consequences) (state, string) {
		list, err := fixture.KubeClientset.CoreV1().Events(fixture.ArgoCDNamespace).List(metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"involvedObject.name":      c.context.name,
				"involvedObject.namespace": fixture.ArgoCDNamespace,
			}).String(),
		})
		if err != nil {
			return failed, err.Error()
		}

		for i := range list.Items {
			event := list.Items[i]
			if event.Reason == reason && strings.Contains(event.Message, message) {
				return succeeded, fmt.Sprintf("found event with reason=%s; message=%s", reason, message)
			}
		}
		return failed, fmt.Sprintf("unable to find event with reason=%s; message=%s", reason, message)
	}
}

// asserts that the last command was successful
func Success(message string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError == nil && strings.Contains(c.actions.lastOutput, message) {
			return succeeded, fmt.Sprintf("found success with message '%s'", c.actions.lastOutput)
		}
		return failed, fmt.Sprintf("expected success with message '%s', got error '%v' message '%s'", message, c.actions.lastError, c.actions.lastOutput)
	}
}

// asserts that the last command was an error with substring match
func Error(message string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError != nil && strings.Contains(c.actions.lastOutput, message) {
			return succeeded, fmt.Sprintf("found error with message '%s'", c.actions.lastOutput)
		}
		return failed, fmt.Sprintf("expected error with message '%s', got error '%v' message '%s'", message, c.actions.lastError, c.actions.lastOutput)
	}
}
