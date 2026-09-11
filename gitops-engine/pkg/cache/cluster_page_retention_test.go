package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/textlogger"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// pagedListResourceInterface serves a fixed set of objects and keeps a handle on every page it
// returned, so tests can check whether the cache retained pointers into a page's backing array.
type pagedListResourceInterface struct {
	*mockResourceInterface
	items []unstructured.Unstructured
	pages []*unstructured.UnstructuredList
}

func (m *pagedListResourceInterface) List(_ context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	page := &unstructured.UnstructuredList{Items: make([]unstructured.Unstructured, len(m.items))}
	copy(page.Items, m.items)
	page.SetResourceVersion("123")
	m.pages = append(m.pages, page)
	return page, nil
}

// TestLoadInitialStateDoesNotRetainListPage asserts that caching a manifest does not pin the list
// page it arrived in, which happens when using pager.EachListItem rather than pager.EachListItemWithAlloc
func TestLoadInitialStateDoesNotRetainListPage(t *testing.T) {
	t.Parallel()

	const (
		pageSize = 10
		// only this pod's manifest gets cached, so the other nine must not be retained with it
		cachedPod = "pod-3"
	)

	items := make([]unstructured.Unstructured, pageSize)
	for i := range items {
		items[i] = unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":            fmt.Sprintf("pod-%d", i),
				"namespace":       "default",
				"uid":             fmt.Sprintf("uid-%d", i),
				"resourceVersion": "123",
			},
		}}
	}
	resClient := &pagedListResourceInterface{mockResourceInterface: &mockResourceInterface{}, items: items}

	cache := &clusterCache{
		listSemaphore:       semaphore.NewWeighted(1),
		listPageSize:        pageSize,
		listPageBufferSize:  1,
		listRetryLimit:      1,
		listRetryFunc:       ListRetryFuncNever,
		log:                 textlogger.NewLogger(textlogger.NewConfig()),
		resources:           map[kube.ResourceKey]*Resource{},
		nsIndex:             map[string]map[kube.ResourceKey]*Resource{},
		parentUIDToChildren: map[types.UID]map[kube.ResourceKey]struct{}{},
		populateResourceInfoHandler: func(un *unstructured.Unstructured, _ bool) (any, bool) {
			return nil, un.GetName() == cachedPod
		},
	}

	api := kube.APIResourceInfo{
		GroupKind:            schema.GroupKind{Kind: "Pod"},
		GroupVersionResource: schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}
	_, err := cache.loadInitialState(t.Context(), api, resClient, "default", false)
	require.NoError(t, err)
	require.Len(t, cache.resources, pageSize)
	require.Len(t, resClient.pages, 1, "test assumes the objects arrive in a single page")

	cached := cache.resources[kube.NewResourceKey("", "Pod", "default", cachedPod)]
	require.NotNil(t, cached)
	require.NotNil(t, cached.Resource, "expected the manifest for %s to be cached", cachedPod)

	// Rather than test that we can run a GC sweep, it's easier to test that the cached manifest's pointer doesn't exist in the page
	// therefore, it shouldn't be retained after a sweep
	page := resClient.pages[0]
	for i := range page.Items {
		assert.NotSame(t, &page.Items[i], cached.Resource,
			"cached manifest points into the list page's backing array, pinning all %d manifests in the page", pageSize)
	}
}
