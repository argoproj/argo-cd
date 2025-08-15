package cache

import (
	"fmt"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

func TestResourceOfGroupKind(t *testing.T) {
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "deploy",
		},
	}
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "service",
		},
	}

	cluster := newCluster(t, deploy, service)
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	resources := cluster.FindResources("", ResourceOfGroupKind("apps", "Deployment"))
	assert.Len(t, resources, 1)
	assert.NotNil(t, resources[kube.NewResourceKey("apps", "Deployment", "", "deploy")])
}

func TestGetNamespaceResources(t *testing.T) {
	defaultNamespaceTopLevel1 := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-guestbook1",
			Namespace: "default",
		},
	}
	defaultNamespaceTopLevel2 := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-guestbook2",
			Namespace: "default",
		},
	}
	kubesystemNamespaceTopLevel2 := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-guestbook3",
			Namespace: "kube-system",
		},
	}

	cluster := newCluster(t, defaultNamespaceTopLevel1, defaultNamespaceTopLevel2, kubesystemNamespaceTopLevel2)
	err := cluster.EnsureSynced()
	require.NoError(t, err)

	resources := cluster.FindResources("default", TopLevelResource)
	assert.Len(t, resources, 2)
	assert.Equal(t, "helm-guestbook1", resources[getResourceKey(t, defaultNamespaceTopLevel1)].Ref.Name)
	assert.Equal(t, "helm-guestbook2", resources[getResourceKey(t, defaultNamespaceTopLevel2)].Ref.Name)

	resources = cluster.FindResources("kube-system", TopLevelResource)
	assert.Len(t, resources, 1)
	assert.Equal(t, "helm-guestbook3", resources[getResourceKey(t, kubesystemNamespaceTopLevel2)].Ref.Name)
}

func ExampleNewClusterCache_inspectNamespaceResources() {
	// kubernetes cluster config here
	config := &rest.Config{}

	clusterCache := NewClusterCache(config,
		// cache default namespace only
		SetNamespaces([]string{"default", "kube-system"}),
		// configure custom logic to cache resources manifest and additional metadata
		SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, _ bool) (info any, cacheManifest bool) {
			// if resource belongs to 'extensions' group then mark if with 'deprecated' label
			if un.GroupVersionKind().Group == "extensions" {
				info = []string{"deprecated"}
			}
			_, ok := un.GetLabels()["acme.io/my-label"]
			// cache whole manifest if resource has label
			cacheManifest = ok
			return
		}),
	)
	// Ensure cluster is synced before using it
	if err := clusterCache.EnsureSynced(); err != nil {
		panic(err)
	}
	// Iterate default namespace resources tree
	for _, root := range clusterCache.FindResources("default", TopLevelResource) {
		clusterCache.IterateHierarchy(root.ResourceKey(), func(resource *Resource, _ map[kube.ResourceKey]*Resource) bool {
			fmt.Printf("resource: %s, info: %v\n", resource.Ref.String(), resource.Info)
			return true
		})
	}
}
