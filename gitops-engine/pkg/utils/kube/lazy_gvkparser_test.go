package kube

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	openapiclient "k8s.io/client-go/openapi"
	"k8s.io/klog/v2/textlogger"
)

func TestGvPathForGVK(t *testing.T) {
	tests := []struct {
		name     string
		gvk      schema.GroupVersionKind
		expected string
	}{
		{
			name:     "core group",
			gvk:      schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			expected: "api/v1",
		},
		{
			name:     "named group",
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: "apis/apps/v1",
		},
		{
			name:     "subdomain group",
			gvk:      schema.GroupVersionKind{Group: "visibility.kueue.x-k8s.io", Version: "v1beta1", Kind: "ClusterQueue"},
			expected: "apis/visibility.kueue.x-k8s.io/v1beta1",
		},
		{
			name:     "CRD group with multiple dots",
			gvk:      schema.GroupVersionKind{Group: "test.example.io", Version: "v2", Kind: "Widget"},
			expected: "apis/test.example.io/v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, gvPathForGVK(tt.gvk))
		})
	}
}

func TestLazyGVKParser_UnknownGV(t *testing.T) {
	// A GVK whose GroupVersion doesn't exist in paths returns (nil, nil)
	// without any network call.
	paths := map[string]openapiclient.GroupVersion{
		"api/v1": &fakeGroupVersion{schemaBytes: validCoreV1Schema},
	}
	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	result, err := parser.Type(schema.GroupVersionKind{Group: "nonexistent.io", Version: "v1", Kind: "Foo"})
	assert.Nil(t, result, "Unknown GV should return nil")
	assert.NoError(t, err, "Unknown GV should not return an error")
}

func TestLazyGVKParser_CachesPerGV(t *testing.T) {
	// After fetching a GV once, subsequent Type() calls for the same GV
	// should use the cached parser (no re-fetch).
	callCount := atomic.Int32{}
	paths := map[string]openapiclient.GroupVersion{
		"api/v1": &countingFakeGroupVersion{
			inner:     &fakeGroupVersion{schemaBytes: validCoreV1Schema},
			callCount: &callCount,
		},
	}
	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	// First call fetches.
	cm, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	require.NoError(t, err)
	require.NotNil(t, cm)
	assert.Equal(t, int32(1), callCount.Load(), "Should have fetched once")

	// Second call for a different kind in the same GV should use cache.
	_, err = parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "Should still be one fetch (cached)")
}

func TestLazyGVKParser_ConcurrentAccess(t *testing.T) {
	// Multiple goroutines requesting the same GV should result in only one
	// fetch thanks to singleflight.
	callCount := atomic.Int32{}
	paths := map[string]openapiclient.GroupVersion{
		"api/v1": &countingFakeGroupVersion{
			inner:     &fakeGroupVersion{schemaBytes: validCoreV1Schema},
			callCount: &callCount,
		},
	}
	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
			assert.NoError(t, err)
			assert.NotNil(t, result)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), callCount.Load(), "Concurrent requests for same GV should result in single fetch")
}

func TestLazyGVKParser_FetchFailureCached(t *testing.T) {
	// A failed fetch should be cached — subsequent calls return the same
	// error without retrying. The cache is invalidated when the cluster
	// cache creates a fresh lazyGVKParser (e.g. on CRD change).
	callCount := atomic.Int32{}
	failOnFirst := &retryableFakeGroupVersion{
		schemas: []schemaResult{
			{err: assert.AnError},     // first call fails
			{data: validCoreV1Schema}, // would succeed if retried
		},
		callCount: &callCount,
	}
	paths := map[string]openapiclient.GroupVersion{
		"api/v1": failOnFirst,
	}
	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	// First call should fail.
	result, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	assert.Nil(t, result, "Should return nil on fetch failure")
	assert.Error(t, err, "Should return error on fetch failure")
	assert.Equal(t, int32(1), callCount.Load())

	// Second call should return cached error without retrying.
	result, err = parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	assert.Nil(t, result, "Should return nil from cached error")
	assert.Error(t, err, "Should return cached error")
	assert.Equal(t, int32(1), callCount.Load(), "Should NOT have retried")
}

func TestLazyGVKParser_ReportedErrorSurfacesInType(t *testing.T) {
	// Errors injected via ReportError should be returned by Type(), even if
	// the schema would otherwise load successfully. This is used by the
	// cluster cache to report list/watch failures (e.g. conversion webhook down).
	paths := map[string]openapiclient.GroupVersion{
		"api/v1": &fakeGroupVersion{schemaBytes: validCoreV1Schema},
	}
	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	webhookErr := fmt.Errorf("failed to list: conversion webhook unavailable")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
	parser.ReportError(gvk, webhookErr)

	// Type() should return the reported error instead of loading the schema.
	result, err := parser.Type(gvk)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, webhookErr)
}

func TestLazyGVKParser_ReportedErrorDoesNotAffectOtherGVs(t *testing.T) {
	// A reported error for one GV should not affect other GVs.
	paths := map[string]openapiclient.GroupVersion{
		"api/v1":       &fakeGroupVersion{schemaBytes: validCoreV1Schema},
		"apis/apps/v1": &fakeGroupVersion{schemaBytes: validAppsV1Schema},
	}
	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	// Report error only for apps/v1
	parser.ReportError(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		fmt.Errorf("webhook down"))

	// core/v1 should still work fine
	cm, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	assert.NoError(t, err)
	assert.NotNil(t, cm)

	// apps/v1 should return the error
	deploy, err := parser.Type(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	assert.Error(t, err)
	assert.Nil(t, deploy)
}

// countingFakeGroupVersion wraps a fakeGroupVersion and counts Schema calls.
type countingFakeGroupVersion struct {
	inner     *fakeGroupVersion
	callCount *atomic.Int32
}

func (f *countingFakeGroupVersion) Schema(contentType string) ([]byte, error) {
	f.callCount.Add(1)
	return f.inner.Schema(contentType)
}

func (f *countingFakeGroupVersion) ServerRelativeURL() string {
	return ""
}

// schemaResult represents one call's return value.
type schemaResult struct {
	data []byte
	err  error
}

// retryableFakeGroupVersion returns different results on successive calls.
type retryableFakeGroupVersion struct {
	schemas   []schemaResult
	callCount *atomic.Int32
}

func (f *retryableFakeGroupVersion) Schema(_ string) ([]byte, error) {
	idx := int(f.callCount.Add(1)) - 1
	if idx >= len(f.schemas) {
		idx = len(f.schemas) - 1
	}
	return f.schemas[idx].data, f.schemas[idx].err
}

func (f *retryableFakeGroupVersion) ServerRelativeURL() string {
	return ""
}
