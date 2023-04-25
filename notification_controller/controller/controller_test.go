package controller

import (
	"context"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestGetAppProj(t *testing.T) {
	testCases := []struct {
		name         string
		app          *unstructured.Unstructured
		appProjItems []*unstructured.Unstructured
		expectedFn   func(proj *unstructured.Unstructured) bool
	}{
		{
			name: "invalid_project_nested_string",
			app: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
			expectedFn: func(proj *unstructured.Unstructured) bool {
				return proj == nil
			},
		},
		{
			name: "non_existent_project",
			app: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"project": "nonexistent",
					},
				},
			},
			appProjItems: []*unstructured.Unstructured{},
			expectedFn: func(proj *unstructured.Unstructured) bool {
				return proj == nil
			},
		},
		{
			name: "valid_project",
			app: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "projectspace",
					},
					"spec": map[string]interface{}{
						"project": "existing",
					},
				},
			},
			appProjItems: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"name":      "existing",
							"namespace": "projectspace",
						},
					},
				},
			},
			expectedFn: func(proj *unstructured.Unstructured) bool {
				if proj == nil {
					return false
				}

				metadata, _, _ := unstructured.NestedMap(proj.Object, "metadata")
				expectedMetadata := map[string]interface{}{
					"name":      "existing",
					"namespace": "projectspace",
				}

				return metadata["name"] == expectedMetadata["name"] &&
					metadata["namespace"] == expectedMetadata["namespace"] &&
					metadata["annotations"] != nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			informer := cache.NewSharedIndexInformer(nil, nil, 0, nil)
			indexer := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})
			for _, item := range tc.appProjItems {
				if err := indexer.Add(item); err != nil {
					t.Fatalf("Failed to add item to indexer: %v", err)
				}
			}
			informer.GetIndexer().Replace(indexer.List(), "test_res_ver")
			proj := getAppProj(tc.app, informer)

			assert.Condition(t, func() bool {
				return tc.expectedFn(proj)
			})
		})
	}
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

	nc := NewController(
		k8sClient,
		dynamicClient,
		nil,
		"default",
		appLabelSelector,
		nil,
		"my-secret",
		"my-configmap",
	)

	assert.NotNil(t, nc)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = nc.Init(ctx)

	assert.NoError(t, err)
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
		appLabelSelector,
		nil,
		"my-secret",
		"my-configmap",
	)

	assert.NotNil(t, nc)

	// Use a short timeout to simulate a timeout during cache synchronization
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err = nc.Init(ctx)

	// Expect an error & add assertion for the error message
	assert.Error(t, err)
	assert.Equal(t, "Timed out waiting for caches to sync", err.Error())
}
