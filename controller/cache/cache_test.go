package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kube/kubetest"
)

const (
	pollInterval = 500 * time.Millisecond
)

func TestWatchClusterResourcesHandlesResourceEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan kube.WatchEvent)
	defer func() {
		cancel()
		close(events)
	}()

	pod := testPod.DeepCopy()

	kubeMock := &kubetest.MockKubectlCmd{
		Resources: []kube.ResourcesBatch{{
			GVK:     pod.GroupVersionKind(),
			Objects: make([]unstructured.Unstructured, 0),
		}},
		Events: events,
	}

	server := "https://test"
	clusterCache := newClusterExt(kubeMock)

	cache := &liveStateCache{
		clusters: map[string]*clusterInfo{server: clusterCache},
		lock:     &sync.Mutex{},
		kubectl:  kubeMock,
	}

	go cache.watchClusterResources(ctx, v1alpha1.Cluster{Server: server})

	assert.False(t, clusterCache.synced())

	events <- kube.WatchEvent{WatchEvent: &watch.Event{Object: pod, Type: watch.Added}}

	err := wait.Poll(pollInterval, wait.ForeverTestTimeout, func() (bool, error) {
		_, hasPod := clusterCache.nodes[kube.GetResourceKey(pod)]
		return hasPod, nil
	})
	assert.Nil(t, err)

	pod.SetResourceVersion("updated-resource-version")
	events <- kube.WatchEvent{WatchEvent: &watch.Event{Object: pod, Type: watch.Modified}}

	err = wait.Poll(pollInterval, wait.ForeverTestTimeout, func() (bool, error) {
		updatedPodInfo, hasPod := clusterCache.nodes[kube.GetResourceKey(pod)]
		return hasPod && updatedPodInfo.resourceVersion == "updated-resource-version", nil
	})
	assert.Nil(t, err)

	events <- kube.WatchEvent{WatchEvent: &watch.Event{Object: pod, Type: watch.Deleted}}

	err = wait.Poll(pollInterval, wait.ForeverTestTimeout, func() (bool, error) {
		_, hasPod := clusterCache.nodes[kube.GetResourceKey(pod)]
		return !hasPod, nil
	})
	assert.Nil(t, err)
}

func TestClusterCacheDroppedOnCreatedDeletedCRD(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan kube.WatchEvent)
	defer func() {
		cancel()
		close(events)
	}()

	kubeMock := &kubetest.MockKubectlCmd{
		Resources: []kube.ResourcesBatch{{
			GVK:     testCRD.GroupVersionKind(),
			Objects: make([]unstructured.Unstructured, 0),
		}},
		Events: events,
	}

	server := "https://test"
	clusterCache := newClusterExt(kubeMock)

	cache := &liveStateCache{
		clusters: map[string]*clusterInfo{server: clusterCache},
		lock:     &sync.Mutex{},
		kubectl:  kubeMock,
	}

	go cache.watchClusterResources(ctx, v1alpha1.Cluster{Server: server})

	err := clusterCache.ensureSynced()
	assert.Nil(t, err)

	events <- kube.WatchEvent{WatchEvent: &watch.Event{Object: testCRD, Type: watch.Added}}
	err = wait.Poll(pollInterval, wait.ForeverTestTimeout, func() (bool, error) {
		cache.lock.Lock()
		defer cache.lock.Unlock()
		_, hasCache := cache.clusters[server]
		return !hasCache, nil
	})
	assert.Nil(t, err)

	cache.clusters[server] = clusterCache

	events <- kube.WatchEvent{WatchEvent: &watch.Event{Object: testCRD, Type: watch.Deleted}}
	err = wait.Poll(pollInterval, wait.ForeverTestTimeout, func() (bool, error) {
		cache.lock.Lock()
		defer cache.lock.Unlock()
		_, hasCache := cache.clusters[server]
		return !hasCache, nil
	})
	assert.Nil(t, err)
}
