package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCheckAppNotInAdditionalNamespaces(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}
	namespace := "argocd"
	var applicationNamespaces []string
	applicationNamespaces = append(applicationNamespaces, "namespace1")
	applicationNamespaces = append(applicationNamespaces, "namespace2")

	// app is in same namespace as controller's namespace
	app.SetNamespace(namespace)
	assert.False(t, checkAppNotInAdditionalNamespaces(app, namespace, applicationNamespaces))

	// app is not in the namespace as controller's namespace, but it is in one of the applicationNamespaces
	app.SetNamespace("namespace2")
	assert.False(t, checkAppNotInAdditionalNamespaces(app, "", applicationNamespaces))

	// app is not in the namespace as controller's namespace, and it is not in any of the applicationNamespaces
	app.SetNamespace("namespace3")
	assert.True(t, checkAppNotInAdditionalNamespaces(app, "", applicationNamespaces))
}
