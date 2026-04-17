package cache

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/sync/semaphore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/pager"
	"k8s.io/klog/v2/textlogger"
)

// mockResourceInterface simulates a dynamic resource interface that returns (nil, error)
type mockResourceInterface struct{}

func (m *mockResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	return errors.New("not implemented")
}

func (m *mockResourceInterface) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return errors.New("not implemented")
}

func (m *mockResourceInterface) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	// This simulates the problematic behavior that caused the original panic:
	// returning (nil, error) instead of (emptyList, error)
	return nil, errors.New("simulated list failure")
}

func (m *mockResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, errors.New("not implemented")
}

func (m *mockResourceInterface) Namespace(string) dynamic.ResourceInterface {
	return m
}

// TestListResourcesNilPointerFix tests that our fix prevents panic when
// resClient.List() returns (nil, error)
func TestListResourcesNilPointerFix(t *testing.T) {
	// Create a cluster cache with proper configuration
	cache := &clusterCache{
		listSemaphore:       semaphore.NewWeighted(1),
		listPageSize:        100,
		listPageBufferSize:  1,
		listRetryLimit:      1,
		listRetryUseBackoff: false,
		listRetryFunc:       ListRetryFuncNever,
		log:                 textlogger.NewLogger(textlogger.NewConfig()),
	}

	// Use our mock that returns (nil, error)
	mockClient := &mockResourceInterface{}

	// This should not panic even though mockClient.List() returns (nil, error)
	_, err := cache.listResources(context.Background(), mockClient, func(listPager *pager.ListPager) error {
		// The fix ensures the pager receives a non-nil UnstructuredList
		// preventing panic in GetContinue()
		return listPager.EachListItem(context.Background(), metav1.ListOptions{}, func(obj runtime.Object) error {
			return nil
		})
	})

	// We expect an error (wrapped), but no panic should occur
	if err == nil {
		t.Fatal("Expected error from listResources due to simulated failure")
	}

	// Check that the error is properly wrapped
	if err.Error() != "failed to list resources: simulated list failure" {
		t.Fatalf("Unexpected error message: %v", err.Error())
	}

	t.Log("Test passed: no panic occurred despite (nil, error) from resClient.List()")
}
