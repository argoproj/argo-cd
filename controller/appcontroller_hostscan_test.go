package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	clustercache "github.com/argoproj/argo-cd/gitops-engine/pkg/cache"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"

	statecache "github.com/argoproj/argo-cd/v3/controller/cache"
	mockstatecache "github.com/argoproj/argo-cd/v3/controller/cache/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// TestGetAppHostsMemoized verifies the per-cluster Node/Pod scan behind
// getAppHosts is computed at most once per hostScanTTL: two getAppHosts calls
// for the same destination cluster must trigger only one IterateResources scan,
// and must return identical host info.
func TestGetAppHostsMemoized(t *testing.T) {
	app := newFakeApp()
	data := &fakeData{apps: []runtime.Object{app, &defaultProj}}
	ctrl := newFakeController(t.Context(), data, nil)

	mockStateCache := &mockstatecache.LiveStateCache{}
	mockStateCache.EXPECT().IterateResources(mock.Anything, mock.MatchedBy(func(callback func(res *clustercache.Resource, info *statecache.ResourceInfo)) bool {
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "minikube", Kind: "Node", APIVersion: "v1"},
		}, &statecache.ResourceInfo{NodeInfo: &statecache.NodeInfo{
			Name:       "minikube",
			SystemInfo: corev1.NodeSystemInfo{OSImage: "debian"},
			Capacity:   map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("5")},
		}})
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "pod1", Kind: kube.PodKind, APIVersion: "v1", Namespace: "default"},
		}, &statecache.ResourceInfo{PodInfo: &statecache.PodInfo{
			NodeName:         "minikube",
			ResourceRequests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("1")},
		}})
		return true
	})).Return(nil).Maybe()
	ctrl.stateCache = mockStateCache

	cluster := &v1alpha1.Cluster{Server: "https://memo.test", Name: "memo"}
	appNodes := []v1alpha1.ResourceNode{{
		ResourceRef: v1alpha1.ResourceRef{Name: "pod1", Namespace: "default", Kind: kube.PodKind},
	}}

	first, err := ctrl.getAppHosts(cluster, app, appNodes)
	require.NoError(t, err)

	second, err := ctrl.getAppHosts(cluster, app, appNodes)
	require.NoError(t, err)

	// The expensive cluster scan must have run only once for the two calls.
	mockStateCache.AssertNumberOfCalls(t, "IterateResources", 1)
	// And the memoized second call must return the same host info.
	assert.Equal(t, first, second)
	assert.Len(t, first, 1)
	assert.Equal(t, "minikube", first[0].Name)
}

// TestGetClusterHostScanPerCluster verifies the memo is keyed per destination
// cluster: distinct clusters each trigger their own scan.
func TestGetClusterHostScanPerCluster(t *testing.T) {
	app := newFakeApp()
	data := &fakeData{apps: []runtime.Object{app, &defaultProj}}
	ctrl := newFakeController(t.Context(), data, nil)

	mockStateCache := &mockstatecache.LiveStateCache{}
	mockStateCache.EXPECT().IterateResources(mock.Anything, mock.Anything).Return(nil).Maybe()
	ctrl.stateCache = mockStateCache

	_, _, err := ctrl.getClusterHostScan(&v1alpha1.Cluster{Server: "https://a.test"})
	require.NoError(t, err)
	_, _, err = ctrl.getClusterHostScan(&v1alpha1.Cluster{Server: "https://b.test"})
	require.NoError(t, err)
	_, _, err = ctrl.getClusterHostScan(&v1alpha1.Cluster{Server: "https://a.test"}) // cached
	require.NoError(t, err)

	// two distinct clusters -> two scans; the repeated one is served from cache
	mockStateCache.AssertNumberOfCalls(t, "IterateResources", 2)
}
