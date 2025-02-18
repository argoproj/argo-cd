package kubectl

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_resolveK8sRequestVerb is adapted from: https://github.com/argoproj/pkg/blob/f5a0a066030558f089fa645dc6546ddc5917bad5/kubeclientmetrics/metric_test.go
func Test_resolveK8sRequestVerb(t *testing.T) {
	testData := []struct {
		testName string
		method   string
		url      string
		expected string
	}{
		{
			testName: "Pod LIST",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces/default/pods",
			expected: "List",
		},
		{
			testName: "Pod Cluster LIST",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/pods",
			expected: "List",
		},
		{
			testName: "Pod GET",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces/default/pods/pod-name-123456",
			expected: "Get",
		},
		{
			testName: "Namespace LIST",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces",
			expected: "List",
		},
		{
			testName: "Namespace GET",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces/default",
			expected: "Get",
		},
		{
			testName: "ReplicaSet LIST",
			method:   "GET",
			url:      "https://127.0.0.1/apis/extensions/v1beta1/namespaces/default/replicasets",
			expected: "List",
		},
		{
			testName: "ReplicaSet Cluster LIST",
			method:   "GET",
			url:      "https://127.0.0.1/apis/apps/v1/replicasets",
			expected: "List",
		},
		{
			testName: "ReplicaSet GET",
			method:   "GET",
			url:      "https://127.0.0.1/apis/extensions/v1beta1/namespaces/default/replicasets/rs-abc123",
			expected: "Get",
		},
		{
			testName: "VirtualService LIST",
			method:   "GET",
			url:      "https://127.0.0.1/apis/networking.istio.io/v1alpha3/namespaces/default/virtualservices",
			expected: "List",
		},
		{
			testName: "VirtualService GET",
			method:   "GET",
			url:      "https://127.0.0.1/apis/networking.istio.io/v1alpha3/namespaces/default/virtualservices/virtual-service",
			expected: "Get",
		},
		{
			testName: "ClusterRole LIST",
			method:   "GET",
			url:      "https://127.0.0.1/apis/rbac.authorization.k8s.io/v1/clusterroles",
			expected: "List",
		},
		{
			testName: "ClusterRole Get",
			method:   "GET",
			url:      "https://127.0.0.1/apis/rbac.authorization.k8s.io/v1/clusterroles/argo-rollouts-clusterrole",
			expected: "Get",
		},
		{
			testName: "CRD List",
			method:   "GET",
			url:      "https://127.0.0.1/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions",
			expected: "List",
		},
		{
			testName: "CRD Get",
			method:   "GET",
			url:      "https://127.0.0.1/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions/dummies.argoproj.io",
			expected: "Get",
		},
		{
			testName: "Resource With Periods Get",
			method:   "GET",
			url:      "https://127.0.0.1/apis/argoproj.io/v1alpha1/namespaces/argocd/applications/my-cluster.cluster.k8s.local",
			expected: "Get",
		},
		{
			testName: "Watch cluster resources",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces?resourceVersion=343003&watch=true",
			expected: "Watch",
		},
		{
			testName: "Watch single cluster resource",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces?fieldSelector=metadata.name%3Ddefault&resourceVersion=0&watch=true",
			expected: "Watch",
		},
		{
			testName: "Watch namespace resources",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces/kube-system/serviceaccounts?resourceVersion=343091&watch=true",
			expected: "Watch",
		},
		{
			testName: "Watch single namespace resource",
			method:   "GET",
			url:      "https://127.0.0.1/api/v1/namespaces/kube-system/serviceaccounts?fieldSelector=metadata.name%3Ddefault&resourceVersion=0&watch=true",
			expected: "Watch",
		},
		{
			testName: "Create single namespace resource",
			method:   "POST",
			url:      "https://127.0.0.1/api/v1/namespaces",
			expected: "Create",
		},
		{
			testName: "Delete single namespace resource",
			method:   "DELETE",
			url:      "https://127.0.0.1/api/v1/namespaces/test",
			expected: "Delete",
		},
		{
			testName: "Patch single namespace resource",
			method:   "PATCH",
			url:      "https://127.0.0.1/api/v1/namespaces/test",
			expected: "Patch",
		},
		{
			testName: "Update single namespace resource",
			method:   "PUT",
			url:      "https://127.0.0.1/api/v1/namespaces/test",
			expected: "Update",
		},
	}

	for _, td := range testData {
		t.Run(td.testName, func(t *testing.T) {
			u, err := url.Parse(td.url)
			require.NoError(t, err)
			info := resolveK8sRequestVerb(*u, td.method)
			assert.Equal(t, td.expected, info)
		})
	}
}
