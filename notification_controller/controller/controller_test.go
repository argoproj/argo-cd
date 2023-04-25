package controller

import (
	"context"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

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
