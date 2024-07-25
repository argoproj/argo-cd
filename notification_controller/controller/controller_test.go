package controller

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestIsAppSyncStatusRefreshed(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logEntry := logger.WithField("", "")

	tests := []struct {
		name          string
		app           *unstructured.Unstructured
		expectedValue bool
	}{
		{
			name: "No OperationState",
			app: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{},
				},
			},
			expectedValue: true,
		},
		{
			name: "No FinishedAt, Completed Phase",
			app: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"operationState": map[string]interface{}{
							"phase": "Succeeded",
						},
					},
				},
			},
			expectedValue: false,
		},
		{
			name: "FinishedAt After ReconciledAt & ObservedAt",
			app: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"operationState": map[string]interface{}{
							"finishedAt": "2021-01-01T01:05:00Z",
							"phase":      "Succeeded",
						},
						"reconciledAt": "2021-01-01T01:02:00Z",
						"observedAt":   "2021-01-01T01:04:00Z",
					},
				},
			},
			expectedValue: false,
		},
		{
			name: "FinishedAt Before ReconciledAt & ObservedAt",
			app: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"operationState": map[string]interface{}{
							"finishedAt": "2021-01-01T01:02:00Z",
							"phase":      "Succeeded",
						},
						"reconciledAt": "2021-01-01T01:04:00Z",
						"observedAt":   "2021-01-01T01:06:00Z",
					},
				},
			},
			expectedValue: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualValue := isAppSyncStatusRefreshed(test.app, logEntry)
			assert.Equal(t, test.expectedValue, actualValue)
		})
	}
}

func TestGetAppProj_invalidProjectNestedString(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}
	informer := cache.NewSharedIndexInformer(nil, nil, 0, nil)
	proj := getAppProj(app, informer)

	assert.Nil(t, proj)
}

func TestInit(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Error registering the resource: %v", err)
	}
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	k8sClient := k8sfake.NewSimpleClientset()
	appLabelSelector := "app=test"

	selfServiceNotificationEnabledFlags := []bool{false, true}
	for _, selfServiceNotificationEnabled := range selfServiceNotificationEnabledFlags {
		nc := NewController(
			k8sClient,
			dynamicClient,
			nil,
			"default",
			[]string{},
			appLabelSelector,
			nil,
			"my-secret",
			"my-configmap",
			selfServiceNotificationEnabled,
		)

		assert.NotNil(t, nc)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = nc.Init(ctx)

		require.NoError(t, err)
	}
}

func TestInitTimeout(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Error registering the resource: %v", err)
	}
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	k8sClient := k8sfake.NewSimpleClientset()
	appLabelSelector := "app=test"

	nc := NewController(
		k8sClient,
		dynamicClient,
		nil,
		"default",
		[]string{},
		appLabelSelector,
		nil,
		"my-secret",
		"my-configmap",
		false,
	)

	assert.NotNil(t, nc)

	// Use a short timeout to simulate a timeout during cache synchronization
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err = nc.Init(ctx)

	// Expect an error & add assertion for the error message
	require.Error(t, err)
	assert.Equal(t, "Timed out waiting for caches to sync", err.Error())
}

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
