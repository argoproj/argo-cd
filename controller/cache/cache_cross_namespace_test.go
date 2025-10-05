package cache

import (
	"testing"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_asResourceNode_cross_namespace_parent(t *testing.T) {
	// Test that a namespaced resource with a cluster-scoped parent
	// correctly sets the parent namespace to empty string

	// Create a Role (namespaced) with an owner reference to a ClusterRole (cluster-scoped)
	roleResource := &clustercache.Resource{
		Ref: corev1.ObjectReference{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
			Namespace:  "my-namespace",
			Name:       "my-role",
		},
		OwnerRefs: []metav1.OwnerReference{
			{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
				Name:       "my-cluster-role",
				UID:        "cluster-role-uid",
			},
		},
	}

	// Create namespace resources map (ClusterRole won't be in here since it's cluster-scoped)
	namespaceResources := map[kube.ResourceKey]*clustercache.Resource{
		// Add some other namespace resources but not the ClusterRole
		{
			Group:     "rbac.authorization.k8s.io",
			Kind:      "Role",
			Namespace: "my-namespace",
			Name:      "other-role",
		}: {
			Ref: corev1.ObjectReference{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "Role",
				Namespace:  "my-namespace",
				Name:       "other-role",
			},
		},
	}

	resNode := asResourceNode(roleResource, namespaceResources)

	// The parent reference should have empty namespace since ClusterRole is cluster-scoped
	assert.Len(t, resNode.ParentRefs, 1)
	assert.Equal(t, "ClusterRole", resNode.ParentRefs[0].Kind)
	assert.Equal(t, "my-cluster-role", resNode.ParentRefs[0].Name)
	assert.Empty(t, resNode.ParentRefs[0].Namespace, "ClusterRole parent should have empty namespace")
}

func Test_asResourceNode_same_namespace_parent(t *testing.T) {
	// Test that a namespaced resource with a namespaced parent in the same namespace
	// correctly sets the parent namespace

	// Create a ReplicaSet with an owner reference to a Deployment (both namespaced)
	rsResource := &clustercache.Resource{
		Ref: corev1.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
			Namespace:  "my-namespace",
			Name:       "my-rs",
		},
		OwnerRefs: []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "my-deployment",
				UID:        "deployment-uid",
			},
		},
	}

	// Create namespace resources map with the Deployment
	deploymentKey := kube.ResourceKey{
		Group:     "apps",
		Kind:      "Deployment",
		Namespace: "my-namespace",
		Name:      "my-deployment",
	}
	namespaceResources := map[kube.ResourceKey]*clustercache.Resource{
		deploymentKey: {
			Ref: corev1.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "my-namespace",
				Name:       "my-deployment",
				UID:        "deployment-uid",
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "my-deployment",
						"namespace": "my-namespace",
						"uid":       "deployment-uid",
					},
				},
			},
		},
	}

	resNode := asResourceNode(rsResource, namespaceResources)

	// The parent reference should have the same namespace
	assert.Len(t, resNode.ParentRefs, 1)
	assert.Equal(t, "Deployment", resNode.ParentRefs[0].Kind)
	assert.Equal(t, "my-deployment", resNode.ParentRefs[0].Name)
	assert.Equal(t, "my-namespace", resNode.ParentRefs[0].Namespace, "Deployment parent should have same namespace")
}
