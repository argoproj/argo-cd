package applicationsets

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"
)

type state = string

const (
	failed    = "failed"
	pending   = "pending"
	succeeded = "succeeded"
)

// Expectation returns succeeded on succes condition, or pending/failed on failure, along with
// a message to describe the success/failure condition.
type Expectation func(c *Consequences) (state state, message string)

// Success asserts that the last command was successful
func Success(message string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError != nil {
			return failed, fmt.Sprintf("error: %v", c.actions.lastError)
		}
		if !strings.Contains(c.actions.lastOutput, message) {
			return failed, fmt.Sprintf("output did not contain '%s'", message)
		}
		return succeeded, fmt.Sprintf("no error and output contained '%s'", message)
	}
}

// Error asserts that the last command was an error with substring match
func Error(message, err string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError == nil {
			return failed, "no error"
		}
		if !strings.Contains(c.actions.lastOutput, message) {
			return failed, fmt.Sprintf("output does not contain '%s'", message)
		}
		if !strings.Contains(c.actions.lastError.Error(), err) {
			return failed, fmt.Sprintf("error does not contain '%s'", message)
		}
		return succeeded, fmt.Sprintf("error '%s'", message)
	}
}

// ApplicationsExist checks whether each of the 'expectedApps' exist in the namespace, and are
// equivalent to provided values.
func ApplicationsExist(expectedApps []v1alpha1.Application) Expectation {
	return func(c *Consequences) (state, string) {
		for _, expectedApp := range expectedApps {
			foundApp := c.app(expectedApp.Name)
			if foundApp == nil {
				return pending, fmt.Sprintf("missing app '%s'", expectedApp.QualifiedName())
			}

			if !appsAreEqual(expectedApp, *foundApp) {
				diff, err := getDiff(filterFields(expectedApp), filterFields(*foundApp))
				if err != nil {
					return failed, err.Error()
				}

				return pending, fmt.Sprintf("apps are not equal: '%s', diff: %s\n", expectedApp.QualifiedName(), diff)
			}
		}

		return succeeded, "all apps successfully found"
	}
}

// ApplicationSetHasConditions checks whether each of the 'expectedConditions' exist in the ApplicationSet status, and are
// equivalent to provided values.
func ApplicationSetHasConditions(applicationSetName string, expectedConditions []v1alpha1.ApplicationSetCondition) Expectation {
	return func(c *Consequences) (state, string) {
		// retrieve the application set
		foundApplicationSet := c.applicationSet(applicationSetName)
		if foundApplicationSet == nil {
			return pending, fmt.Sprintf("application set '%s' not found", applicationSetName)
		}

		if !conditionsAreEqual(&expectedConditions, &foundApplicationSet.Status.Conditions) {
			diff, err := getConditionDiff(expectedConditions, foundApplicationSet.Status.Conditions)
			if err != nil {
				return failed, err.Error()
			}
			return pending, fmt.Sprintf("application set conditions are not equal: '%s', diff: %s\n", expectedConditions, diff)
		}
		return succeeded, "application set successfully found"
	}
}

// ApplicationsDoNotExist checks that each of the 'expectedApps' no longer exist in the namespace
func ApplicationsDoNotExist(expectedApps []v1alpha1.Application) Expectation {
	return func(c *Consequences) (state, string) {
		for _, expectedApp := range expectedApps {
			foundApp := c.app(expectedApp.Name)
			if foundApp != nil {
				return pending, fmt.Sprintf("app '%s' should no longer exist", expectedApp.QualifiedName())
			}
		}

		return succeeded, "all apps do not exist"
	}
}

// Pod checks whether a specified condition is true for any of the pods in the namespace
func Pod(predicate func(p corev1.Pod) bool) Expectation {
	return func(c *Consequences) (state, string) {
		pods, err := pods(utils.ApplicationsResourcesNamespace)
		if err != nil {
			return failed, err.Error()
		}
		for _, pod := range pods.Items {
			if predicate(pod) {
				return succeeded, fmt.Sprintf("pod predicate matched pod named '%s'", pod.GetName())
			}
		}
		return pending, "pod predicate does not match pods"
	}
}

func pods(namespace string) (*corev1.PodList, error) {
	fixtureClient := utils.GetE2EFixtureK8sClient()

	pods, err := fixtureClient.KubeClientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	return pods, err
}

// getDiff returns a string containing a comparison result of two applications (for test output/debug purposes)
func getDiff(orig, new v1alpha1.Application) (string, error) {
	bytes, _, err := diff.CreateTwoWayMergePatch(orig, new, orig)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// getConditionDiff returns a string containing a comparison result of two ApplicationSetCondition (for test output/debug purposes)
func getConditionDiff(orig, new []v1alpha1.ApplicationSetCondition) (string, error) {
	if len(orig) != len(new) {
		return fmt.Sprintf("mismatch between condition sizes: %v %v", len(orig), len(new)), nil
	}

	var bytes []byte

	for index := range orig {
		b, _, err := diff.CreateTwoWayMergePatch(orig[index], new[index], orig[index])
		if err != nil {
			return "", err
		}
		bytes = append(bytes, b...)
	}

	return string(bytes), nil
}

// filterFields returns a copy of Application, but with unnecessary (for testing) fields removed
func filterFields(input v1alpha1.Application) v1alpha1.Application {
	spec := input.Spec

	metaCopy := input.ObjectMeta.DeepCopy()

	output := v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      metaCopy.Labels,
			Annotations: metaCopy.Annotations,
			Name:        metaCopy.Name,
			Namespace:   metaCopy.Namespace,
			Finalizers:  metaCopy.Finalizers,
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				Path:           spec.GetSource().Path,
				RepoURL:        spec.GetSource().RepoURL,
				TargetRevision: spec.GetSource().TargetRevision,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    spec.Destination.Server,
				Name:      spec.Destination.Name,
				Namespace: spec.Destination.Namespace,
			},
			Project: spec.Project,
		},
	}

	return output
}

// filterConditionFields returns a copy of ApplicationSetCondition, but with unnecessary (for testing) fields removed
func filterConditionFields(input *[]v1alpha1.ApplicationSetCondition) *[]v1alpha1.ApplicationSetCondition {
	var filteredConditions []v1alpha1.ApplicationSetCondition
	for _, condition := range *input {
		newCondition := &v1alpha1.ApplicationSetCondition{
			Type:    condition.Type,
			Status:  condition.Status,
			Message: condition.Message,
			Reason:  condition.Reason,
		}
		filteredConditions = append(filteredConditions, *newCondition)
	}

	return &filteredConditions
}

// appsAreEqual returns true if the apps are equal, comparing only fields of interest
func appsAreEqual(one v1alpha1.Application, two v1alpha1.Application) bool {
	return reflect.DeepEqual(filterFields(one), filterFields(two))
}

// conditionsAreEqual returns true if the appset status conditions are equal, comparing only fields of interest
func conditionsAreEqual(one, two *[]v1alpha1.ApplicationSetCondition) bool {
	return reflect.DeepEqual(filterConditionFields(one), filterConditionFields(two))
}
