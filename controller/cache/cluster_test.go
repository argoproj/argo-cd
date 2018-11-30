package cache

import (
	"sort"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kube/kubetest"
	log "github.com/sirupsen/logrus"
)

func strToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	errors.CheckError(err)
	return &unstructured.Unstructured{Object: obj}
}

func mustToUnstructured(obj interface{}) *unstructured.Unstructured {
	un, err := kube.ToUnstructured(obj)
	errors.CheckError(err)
	return un
}

var (
	testPod = strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    resourceVersion: "123"`)

	testRS = strToUnstructured(`
  apiVersion: v1
  apiVersion: extensions/v1beta1
  kind: ReplicaSet
  metadata:
    name: helm-guestbook-rs
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: Deployment
      name: helm-guestbook
    resourceVersion: "123"`)

	testDeploy = strToUnstructured(`
  apiVersion: extensions/v1beta1
  kind: Deployment
  metadata:
    labels:
      applications.argoproj.io/app-name: helm-guestbook
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"`)
)

func newCluster(resources ...*unstructured.Unstructured) *clusterInfo {
	return &clusterInfo{
		lock:         &sync.Mutex{},
		nodes:        make(map[kube.ResourceKey]*node),
		onAppUpdated: func(appName string) {},
		kubectl: kubetest.MockKubectlCmd{
			Resources: resources,
		},
		cluster:  &appv1.Cluster{},
		syncTime: nil,
		syncLock: &sync.Mutex{},
		apis:     make(map[schema.GroupVersionKind]v1.APIResource),
		log:      log.WithField("cluster", "test"),
	}
}

func TestGetChildren(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	rsChildren := cluster.getChildren(testRS)
	assert.Equal(t, []appv1.ResourceNode{{
		Kind:            "Pod",
		Namespace:       "default",
		Name:            "helm-guestbook-pod",
		Group:           "",
		Version:         "v1",
		Tags:            []string{"0/0"},
		Children:        make([]appv1.ResourceNode, 0),
		ResourceVersion: "123",
	}}, rsChildren)
	deployChildren := cluster.getChildren(testDeploy)

	assert.Equal(t, []appv1.ResourceNode{{
		Kind:            "ReplicaSet",
		Namespace:       "default",
		Name:            "helm-guestbook-rs",
		Group:           "extensions",
		Version:         "v1beta1",
		ResourceVersion: "123",
		Children:        rsChildren,
		Tags:            []string{},
	}}, deployChildren)
}

func TestGetManagedLiveObjs(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	targetDeploy := strToUnstructured(`
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: helm-guestbook
  labels:
    app: helm-guestbook`)

	managedObjs, err := cluster.getManagedLiveObjs(&appv1.Application{
		ObjectMeta: v1.ObjectMeta{Name: "helm-guestbook"},
		Spec: appv1.ApplicationSpec{
			Destination: appv1.ApplicationDestination{
				Namespace: "default",
			},
		},
	}, []*unstructured.Unstructured{targetDeploy})
	assert.Nil(t, err)
	assert.Equal(t, managedObjs, map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("apps", "Deployment", "default", "helm-guestbook"): testDeploy,
	})
}

func TestChildDeletedEvent(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	err = cluster.processEvent(watch.Deleted, testPod)
	assert.Nil(t, err)

	rsChildren := cluster.getChildren(testRS)
	assert.Equal(t, []appv1.ResourceNode{}, rsChildren)
}

func TestProcessNewChildEvent(t *testing.T) {
	cluster := newCluster(testPod, testRS, testDeploy)
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	newPod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: helm-guestbook-pod2
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    resourceVersion: "123"`)

	err = cluster.processEvent(watch.Added, newPod)
	assert.Nil(t, err)

	rsChildren := cluster.getChildren(testRS)
	sort.Slice(rsChildren, func(i, j int) bool {
		return strings.Compare(rsChildren[i].Name, rsChildren[j].Name) < 0
	})
	assert.Equal(t, []appv1.ResourceNode{{
		Kind:            "Pod",
		Namespace:       "default",
		Name:            "helm-guestbook-pod",
		Group:           "",
		Version:         "v1",
		Tags:            []string{"0/0"},
		Children:        make([]appv1.ResourceNode, 0),
		ResourceVersion: "123",
	}, {
		Kind:            "Pod",
		Namespace:       "default",
		Name:            "helm-guestbook-pod2",
		Group:           "",
		Version:         "v1",
		Tags:            []string{"0/0"},
		Children:        make([]appv1.ResourceNode, 0),
		ResourceVersion: "123",
	}}, rsChildren)
}

func TestUpdateResourceTags(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta:   v1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: v1.ObjectMeta{Name: "testPod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "test",
			}},
		},
	}
	cluster := newCluster(mustToUnstructured(pod))

	err := cluster.ensureSynced()
	assert.Nil(t, err)

	podNode := cluster.nodes[kube.GetResourceKey(mustToUnstructured(pod))]

	assert.NotNil(t, podNode)
	assert.Equal(t, []string{"0/1"}, podNode.tags)

	pod.Status = corev1.PodStatus{
		ContainerStatuses: []corev1.ContainerStatus{{
			State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: -1,
				},
			},
		}},
	}
	err = cluster.processEvent(watch.Modified, mustToUnstructured(pod))
	assert.Nil(t, err)

	podNode = cluster.nodes[kube.GetResourceKey(mustToUnstructured(pod))]

	assert.NotNil(t, podNode)
	assert.Equal(t, []string{"ExitCode:-1", "0/1"}, podNode.tags)
}

func TestUpdateAppResource(t *testing.T) {
	updatesReceived := make([]string, 0)
	cluster := newCluster(testPod, testRS, testDeploy)
	cluster.onAppUpdated = func(appName string) {
		updatesReceived = append(updatesReceived, appName)
	}

	err := cluster.ensureSynced()
	assert.Nil(t, err)

	err = cluster.processEvent(watch.Modified, mustToUnstructured(testPod))
	assert.Nil(t, err)

	assert.Equal(t, []string{"helm-guestbook"}, updatesReceived)
}

func TestUpdateRootAppResource(t *testing.T) {
	updatesReceived := make([]string, 0)
	cluster := newCluster(testPod, testRS, testDeploy)
	cluster.onAppUpdated = func(appName string) {
		updatesReceived = append(updatesReceived, appName)
	}
	err := cluster.ensureSynced()
	assert.Nil(t, err)

	for k := range cluster.nodes {
		assert.Equal(t, "helm-guestbook", cluster.nodes[k].appName)
	}

	updatedDeploy := testDeploy.DeepCopy()
	updatedDeploy.SetLabels(map[string]string{common.LabelApplicationName: "helm-guestbook2"})

	err = cluster.processEvent(watch.Modified, updatedDeploy)
	assert.Nil(t, err)

	assert.Equal(t, []string{"helm-guestbook", "helm-guestbook2"}, updatesReceived)
	for k := range cluster.nodes {
		assert.Equal(t, "helm-guestbook2", cluster.nodes[k].appName)
	}
}
