package controller

import (
	"sync/atomic"
	"testing"
	"time"

	clustercache "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	statecache "github.com/argoproj/argo-cd/v3/controller/cache"
	mockstatecache "github.com/argoproj/argo-cd/v3/controller/cache/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/test"
)

func TestGetAppHostsScansEachClusterOncePerTTL(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		configMapData: map[string]string{
			"application.allowedNodeLabels": "label1,label2",
		},
	}, nil)

	var scans atomic.Int32
	mockStateCache := &mockstatecache.LiveStateCache{}
	mockStateCache.EXPECT().IterateResources(mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		scans.Add(1)
		callback := args.Get(1).(func(*clustercache.Resource, *statecache.ResourceInfo))
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "minikube", Kind: "Node", APIVersion: "v1"},
		}, &statecache.ResourceInfo{NodeInfo: &statecache.NodeInfo{
			Name:       "minikube",
			SystemInfo: corev1.NodeSystemInfo{OSImage: "debian"},
			Capacity:   map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("5")},
			Labels:     map[string]string{"label1": "value1"},
		}})
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "pod1", Kind: kube.PodKind, APIVersion: "v1", Namespace: "default"},
		}, &statecache.ResourceInfo{PodInfo: &statecache.PodInfo{
			NodeName:         "minikube",
			ResourceRequests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("1")},
		}})
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "pod2", Kind: kube.PodKind, APIVersion: "v1", Namespace: "default"},
		}, &statecache.ResourceInfo{PodInfo: &statecache.PodInfo{
			NodeName:         "minikube",
			ResourceRequests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("2")},
		}})
	}).Return(nil).Twice()
	ctrl.stateCache = mockStateCache
	ctrl.hostScans.TTL = time.Hour

	nodes := []v1alpha1.ResourceNode{{Name: "pod1", Namespace: "default", Kind: kube.PodKind}}
	cluster := &v1alpha1.Cluster{Server: "https://cluster-a", Name: "a"}
	first, err := ctrl.getAppHosts(cluster, app, nodes)
	require.NoError(t, err)
	second, err := ctrl.getAppHosts(cluster, app, nodes)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, []v1alpha1.HostInfo{{
		Name:       "minikube",
		SystemInfo: corev1.NodeSystemInfo{OSImage: "debian"},
		ResourcesInfo: []v1alpha1.HostResourceInfo{{
			ResourceName: corev1.ResourceCPU, Capacity: 5000, RequestedByApp: 1000, RequestedByNeighbors: 2000,
		}},
		Labels: map[string]string{"label1": "value1"},
	}}, first)
	assert.Equal(t, int32(1), scans.Load())

	_, err = ctrl.getAppHosts(&v1alpha1.Cluster{Server: "https://cluster-b", Name: "b"}, app, nodes)
	require.NoError(t, err)
	assert.Equal(t, int32(2), scans.Load())
}
